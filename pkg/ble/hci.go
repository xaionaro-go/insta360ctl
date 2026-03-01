package ble

import (
	"context"
	"fmt"
	"strings"
	"unsafe"

	"github.com/facebookincubator/go-belt/tool/logger"
	"github.com/xaionaro-go/gatt"
	"github.com/xaionaro-go/gatt/linux/gioctl"
	"github.com/xaionaro-go/gatt/linux/socket"
	"golang.org/x/sys/unix"
)

// HCIAdapter implements Adapter using raw HCI sockets (xaionaro-go/gatt).
type HCIAdapter struct {
	device      gatt.Device
	peripherals map[string]gatt.Peripheral
	connectedCh chan hciConnEvent
}

type hciConnEvent struct {
	periph gatt.Peripheral
	err    error
}

// NewHCIAdapter creates a new BLE adapter using raw HCI sockets.
func NewHCIAdapter(ctx context.Context) (*HCIAdapter, error) {
	d, err := gatt.NewDevice(ctx,
		gatt.LnxMaxConnections(1),
		gatt.LnxDeviceID(-1, true),
	)
	if err != nil {
		logger.Warnf(ctx, "BLE device open failed (%v), attempting HCI reset...", err)
		if resetErr := resetHCI(ctx); resetErr != nil {
			logger.Warnf(ctx, "HCI reset failed: %v", resetErr)
			return nil, fmt.Errorf("failed to open BLE device: %w (HCI reset also failed: %v)", err, resetErr)
		}
		d, err = gatt.NewDevice(ctx,
			gatt.LnxMaxConnections(1),
			gatt.LnxDeviceID(-1, true),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to open BLE device after HCI reset: %w", err)
		}
	}

	return &HCIAdapter{
		device:      d,
		peripherals: make(map[string]gatt.Peripheral),
		connectedCh: make(chan hciConnEvent, 1),
	}, nil
}

// GattDevice returns the underlying gatt.Device (for the remote package).
func (a *HCIAdapter) GattDevice() gatt.Device {
	return a.device
}

func (a *HCIAdapter) Scan(ctx context.Context, callback func(ScanResult)) error {
	errCh := make(chan error, 1)

	a.device.Handle(ctx,
		gatt.PeripheralDiscovered(func(ctx context.Context, periph gatt.Peripheral, adv *gatt.Advertisement, rssi int) {
			a.peripherals[periph.ID()] = periph
			callback(ScanResult{
				Address:   periph.ID(),
				LocalName: adv.LocalName,
				RSSI:      rssi,
			})
		}),
		gatt.PeripheralConnected(func(ctx context.Context, periph gatt.Peripheral, err error) {
			a.peripherals[periph.ID()] = periph
			select {
			case a.connectedCh <- hciConnEvent{periph: periph, err: err}:
			default:
			}
		}),
		gatt.PeripheralDisconnected(func(ctx context.Context, periph gatt.Peripheral, err error) {
			logger.Infof(ctx, "peripheral %s disconnected", periph.ID())
		}),
	)

	err := a.device.Start(ctx, func(ctx context.Context, d gatt.Device, s gatt.State) {
		switch s {
		case gatt.StatePoweredOn:
			if err := d.Scan(ctx, nil, false); err != nil {
				errCh <- fmt.Errorf("unable to start scanning: %w", err)
			}
		default:
			errCh <- fmt.Errorf("unexpected BLE state: %s", s)
		}
	})
	if err != nil {
		return fmt.Errorf("unable to initialize BLE: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (a *HCIAdapter) StopScan() error {
	return a.device.StopScanning()
}

func (a *HCIAdapter) Connect(ctx context.Context, addr string) (Peripheral, error) {
	periph, ok := a.peripherals[addr]
	if !ok {
		return nil, fmt.Errorf("device %s not found in scan results", addr)
	}

	a.device.Connect(ctx, periph)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case evt := <-a.connectedCh:
		if evt.err != nil {
			return nil, fmt.Errorf("connection failed: %w", evt.err)
		}
		return &hciPeripheral{
			adapter: a,
			periph:  evt.periph,
		}, nil
	}
}

func (a *HCIAdapter) Close(ctx context.Context) error {
	return nil
}

// --- Peripheral ---

type hciPeripheral struct {
	adapter  *HCIAdapter
	periph   gatt.Peripheral
	services []*gatt.Service
}

func (p *hciPeripheral) ID() string { return p.periph.ID() }

func (p *hciPeripheral) DiscoverServices(ctx context.Context) ([]Service, error) {
	_, _ = p.periph.DiscoverIncludedServices(ctx, nil, nil)

	svcs, err := p.periph.DiscoverServices(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to discover services: %w", err)
	}
	p.services = svcs

	result := make([]Service, len(svcs))
	for i, s := range svcs {
		result[i] = &hciService{
			adapter: p.adapter,
			periph:  p.periph,
			svc:     s,
		}
	}
	return result, nil
}

func (p *hciPeripheral) SetMTU(ctx context.Context, mtu uint16) error {
	return p.periph.SetMTU(ctx, mtu)
}

func (p *hciPeripheral) Disconnect(ctx context.Context) error {
	p.adapter.device.CancelConnection(ctx, p.periph)
	return nil
}

// --- Service ---

type hciService struct {
	adapter *HCIAdapter
	periph  gatt.Peripheral
	svc     *gatt.Service
}

func (s *hciService) UUID() string {
	return strings.ToLower(s.svc.UUID().String())
}

func (s *hciService) DiscoverCharacteristics(ctx context.Context) ([]Characteristic, error) {
	chars, err := s.periph.DiscoverCharacteristics(ctx, nil, s.svc)
	if err != nil {
		return nil, fmt.Errorf("failed to discover characteristics: %w", err)
	}

	result := make([]Characteristic, len(chars))
	for i, c := range chars {
		result[i] = &hciCharacteristic{
			periph: s.periph,
			char:   c,
		}
	}
	return result, nil
}

// --- Characteristic ---

type hciCharacteristic struct {
	periph gatt.Peripheral
	char   *gatt.Characteristic
}

func (c *hciCharacteristic) UUID() string {
	return strings.ToLower(c.char.UUID().String())
}

func (c *hciCharacteristic) Write(data []byte, withResponse bool) error {
	return c.periph.WriteCharacteristic(context.Background(), c.char, data, !withResponse)
}

func (c *hciCharacteristic) Read() ([]byte, error) {
	return c.periph.ReadCharacteristic(context.Background(), c.char)
}

func (c *hciCharacteristic) EnableNotifications(callback func([]byte)) error {
	// Discover descriptors first (needed for CCCD).
	_, _ = c.periph.DiscoverDescriptors(context.Background(), nil, c.char)

	return c.periph.SetNotifyValue(context.Background(), c.char, func(ch *gatt.Characteristic, b []byte, err error) {
		if err != nil {
			return
		}
		callback(b)
	})
}

func (c *hciCharacteristic) Properties() CharProps {
	p := c.char.Properties()
	var props CharProps
	if p&gatt.CharRead != 0 {
		props |= CharPropRead
	}
	if p&gatt.CharWrite != 0 {
		props |= CharPropWrite
	}
	if p&gatt.CharWriteNR != 0 {
		props |= CharPropWriteNR
	}
	if p&gatt.CharNotify != 0 {
		props |= CharPropNotify
	}
	if p&gatt.CharIndicate != 0 {
		props |= CharPropIndicate
	}
	return props
}

// --- HCI Reset ---

const (
	ioctlSize     = uintptr(4)
	typeHCI       = 72 // 'H'
	hciMaxDevices = 16
)

var (
	hciUpDevice      = gioctl.IoW(typeHCI, 201, ioctlSize)
	hciDownDevice    = gioctl.IoW(typeHCI, 202, ioctlSize)
	hciGetDeviceList = gioctl.IoR(typeHCI, 210, ioctlSize)
)

type devRequest struct {
	id  uint16
	opt uint32
}

type devListRequest struct {
	devNum     uint16
	devRequest [hciMaxDevices]devRequest
}

func resetHCI(ctx context.Context) error {
	fd, err := socket.Socket(socket.AF_BLUETOOTH, unix.SOCK_RAW, socket.BTPROTO_HCI)
	if err != nil {
		return fmt.Errorf("failed to create HCI socket: %w", err)
	}
	defer unix.Close(fd)

	req := devListRequest{devNum: hciMaxDevices}
	if err := gioctl.Ioctl(uintptr(fd), hciGetDeviceList, uintptr(unsafe.Pointer(&req))); err != nil {
		return fmt.Errorf("failed to list HCI devices: %w", err)
	}

	if req.devNum == 0 {
		return fmt.Errorf("no HCI devices found")
	}

	for i := range int(req.devNum) {
		devID := uintptr(req.devRequest[i].id)
		logger.Debugf(ctx, "resetting HCI device %d...", devID)
		_ = gioctl.Ioctl(uintptr(fd), hciDownDevice, devID)
		if err := gioctl.Ioctl(uintptr(fd), hciUpDevice, devID); err != nil {
			logger.Warnf(ctx, "failed to bring up HCI device %d: %v", devID, err)
			continue
		}
		logger.Infof(ctx, "HCI device %d reset successfully", devID)
	}
	return nil
}
