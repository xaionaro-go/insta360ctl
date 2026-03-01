package ble

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/facebookincubator/go-belt/tool/logger"
	"github.com/godbus/dbus/v5"
	"tinygo.org/x/bluetooth"
)

// DBusAdapter implements Adapter using tinygo.org/x/bluetooth (BlueZ D-Bus).
type DBusAdapter struct {
	adapter *bluetooth.Adapter

	mu      sync.Mutex
	devices map[string]bluetooth.ScanResult // keyed by address
}

// NewDBusAdapter creates a new BLE adapter using the BlueZ D-Bus backend.
// adapterID selects the HCI adapter (e.g. "hci0", "hci1"). Empty string uses the default.
func NewDBusAdapter(ctx context.Context, adapterID string) (*DBusAdapter, error) {
	var adapter *bluetooth.Adapter
	if adapterID == "" {
		adapter = bluetooth.DefaultAdapter
	} else {
		adapter = bluetooth.NewAdapter(adapterID)
	}
	if err := adapter.Enable(); err != nil {
		return nil, fmt.Errorf("failed to enable BLE adapter %s: %w", adapterID, err)
	}

	// Start a D-Bus signal sniffer to verify signal delivery.
	go dbusSignalSniffer(ctx)

	return &DBusAdapter{
		adapter: adapter,
		devices: make(map[string]bluetooth.ScanResult),
	}, nil
}

// dbusSignalSniffer monitors ALL D-Bus signals on the system bus.
// This helps debug whether PropertiesChanged signals are being delivered.
func dbusSignalSniffer(ctx context.Context) {
	conn, err := dbus.SystemBus()
	if err != nil {
		logger.Warnf(ctx, "[dbus-sniffer] failed to connect to system bus: %v", err)
		return
	}

	// Subscribe to all PropertiesChanged signals from BlueZ.
	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
	); err != nil {
		logger.Warnf(ctx, "[dbus-sniffer] failed to add match signal: %v", err)
		return
	}

	sigCh := make(chan *dbus.Signal, 100)
	conn.Signal(sigCh)
	logger.Infof(ctx, "[dbus-sniffer] listening for ALL PropertiesChanged signals on system bus...")

	for {
		select {
		case <-ctx.Done():
			conn.RemoveSignal(sigCh)
			return
		case sig := <-sigCh:
			if sig == nil {
				return
			}
			if sig.Name == "org.freedesktop.DBus.Properties.PropertiesChanged" && len(sig.Body) >= 2 {
				ifaceName, _ := sig.Body[0].(string)
				if ifaceName == "org.bluez.GattCharacteristic1" {
					changes, _ := sig.Body[1].(map[string]dbus.Variant)
					if val, ok := changes["Value"]; ok {
						data, _ := val.Value().([]byte)
						logger.Infof(ctx, "[dbus-sniffer] CHARACTERISTIC VALUE CHANGED: path=%s data=%X", sig.Path, data)
					} else {
						logger.Debugf(ctx, "[dbus-sniffer] characteristic property changed: path=%s changes=%v", sig.Path, changes)
					}
				} else if ifaceName == "org.bluez.Device1" {
					logger.Debugf(ctx, "[dbus-sniffer] device property changed: path=%s", sig.Path)
				}
			} else {
				logger.Debugf(ctx, "[dbus-sniffer] signal: name=%s path=%s", sig.Name, sig.Path)
			}
		}
	}
}

func (a *DBusAdapter) Scan(ctx context.Context, callback func(ScanResult)) error {
	errCh := make(chan error, 1)

	go func() {
		errCh <- a.adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
			addr := result.Address.String()
			name := result.LocalName()
			rssi := int(result.RSSI)

			a.mu.Lock()
			a.devices[addr] = result
			a.mu.Unlock()

			callback(ScanResult{
				Address:   addr,
				LocalName: name,
				RSSI:      rssi,
			})
		})
	}()

	select {
	case <-ctx.Done():
		_ = a.adapter.StopScan()
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (a *DBusAdapter) StopScan() error {
	return a.adapter.StopScan()
}

func (a *DBusAdapter) Connect(ctx context.Context, addr string) (Peripheral, error) {
	a.mu.Lock()
	scanResult, ok := a.devices[addr]
	a.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("device %s not found in scan results", addr)
	}

	device, err := a.adapter.Connect(scanResult.Address, bluetooth.ConnectionParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	return &dbusPeripheral{
		adapter: a,
		device:  device,
		addr:    addr,
	}, nil
}

func (a *DBusAdapter) Close(ctx context.Context) error {
	return nil
}

// --- Peripheral ---

type dbusPeripheral struct {
	adapter *DBusAdapter
	device  bluetooth.Device
	addr    string

	services []dbusService
}

func (p *dbusPeripheral) ID() string { return p.addr }

func (p *dbusPeripheral) DiscoverServices(ctx context.Context) ([]Service, error) {
	svcs, err := p.device.DiscoverServices(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to discover services: %w", err)
	}

	logger.Debugf(ctx, "[dbus-discovery] found %d services for device %s", len(svcs), p.addr)
	p.services = make([]dbusService, len(svcs))
	result := make([]Service, len(svcs))
	for i, s := range svcs {
		logger.Debugf(ctx, "[dbus-discovery] service %d: uuid=%s", i, s.UUID().String())
		p.services[i] = dbusService{
			adapter: p.adapter,
			svc:     s,
		}
		result[i] = &p.services[i]
	}
	return result, nil
}

func (p *dbusPeripheral) SetMTU(ctx context.Context, mtu uint16) error {
	// BlueZ handles MTU negotiation automatically.
	// The MTU is negotiated during connection and can be read from characteristics.
	return nil
}

func (p *dbusPeripheral) Disconnect(ctx context.Context) error {
	return p.device.Disconnect()
}

// --- Service ---

type dbusService struct {
	adapter *DBusAdapter
	svc     bluetooth.DeviceService
}

func (s *dbusService) UUID() string {
	return strings.ToLower(s.svc.UUID().String())
}

func (s *dbusService) DiscoverCharacteristics(ctx context.Context) ([]Characteristic, error) {
	chars, err := s.svc.DiscoverCharacteristics(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to discover characteristics: %w", err)
	}

	result := make([]Characteristic, len(chars))
	for i := range chars {
		result[i] = &dbusCharacteristic{
			adapter: s.adapter,
			char:    &chars[i],
		}
	}
	return result, nil
}

// --- Characteristic ---

type dbusCharacteristic struct {
	adapter *DBusAdapter
	char    *bluetooth.DeviceCharacteristic
}

func (c *dbusCharacteristic) UUID() string {
	return strings.ToLower(c.char.UUID().String())
}

func (c *dbusCharacteristic) Write(data []byte, withResponse bool) error {
	ctx := context.Background()
	logger.Debugf(ctx, "[dbus-write] uuid=%s len=%d withResponse=%v data=%X",
		c.UUID(), len(data), withResponse, data)

	// tinygo's WriteWithoutResponse calls BlueZ WriteValue with no options.
	// BlueZ picks write-request or write-command based on characteristic properties.
	_, err := c.char.WriteWithoutResponse(data)
	if err != nil {
		logger.Warnf(ctx, "[dbus-write] FAILED: uuid=%s err=%v", c.UUID(), err)
	} else {
		logger.Debugf(ctx, "[dbus-write] OK: uuid=%s", c.UUID())
	}
	return err
}

func (c *dbusCharacteristic) Read() ([]byte, error) {
	buf := make([]byte, 512)
	n, err := c.char.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (c *dbusCharacteristic) EnableNotifications(callback func([]byte)) error {
	ctx := context.Background()
	logger.Debugf(ctx, "[dbus-notify] subscribing to notifications: uuid=%s", c.UUID())

	wrappedCallback := func(data []byte) {
		logger.Debugf(ctx, "[dbus-notify] RECEIVED notification: uuid=%s len=%d data=%X", c.UUID(), len(data), data)
		callback(data)
	}

	err := c.char.EnableNotifications(wrappedCallback)
	if err != nil {
		logger.Warnf(ctx, "[dbus-notify] FAILED to subscribe: uuid=%s err=%v", c.UUID(), err)
	} else {
		logger.Infof(ctx, "[dbus-notify] subscribed OK: uuid=%s", c.UUID())
	}
	return err
}

func (c *dbusCharacteristic) Properties() CharProps {
	// tinygo doesn't expose properties directly via the Linux backend.
	// Return all capabilities and let the caller try operations.
	return CharPropRead | CharPropWrite | CharPropWriteNR | CharPropNotify
}
