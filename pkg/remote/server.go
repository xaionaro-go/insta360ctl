package remote

import (
	"context"
	"fmt"
	"sync"

	"github.com/facebookincubator/go-belt/tool/logger"
	"github.com/xaionaro-go/gatt"
	"github.com/xaionaro-go/insta360ctl/pkg/protocol"
)

// Server emulates an Insta360 GPS Remote (Architecture A).
// It sets up a GATT server with the CE80 and D0FF services,
// allowing an Insta360 camera to connect and receive commands.
type Server struct {
	device gatt.Device
	name   string

	mu       sync.Mutex
	notifier gatt.Notifier
	running  bool
}

// NewServer creates a new GPS Remote server.
func NewServer(device gatt.Device, name string) *Server {
	if name == "" {
		name = "Insta360 GPS Remote"
	}
	return &Server{
		device: device,
		name:   name,
	}
}

// shortUUID converts a uint16 short UUID to a gatt.UUID.
func shortUUID(id uint16) gatt.UUID {
	return gatt.UUID16(id)
}

// fullUUID converts a 128-bit UUID string to a gatt.UUID.
func fullUUID(s string) gatt.UUID {
	return gatt.MustParseUUID(s)
}

// Start sets up GATT services and begins advertising.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}
	s.running = true
	s.mu.Unlock()

	if err := s.setupServices(ctx); err != nil {
		return fmt.Errorf("failed to setup services: %w", err)
	}

	return nil
}

func (s *Server) setupServices(ctx context.Context) error {
	// Primary CE80 service — GPS Remote control
	primarySvc := gatt.NewService(shortUUID(protocol.UUIDRemoteService))

	// CE81 — Write characteristic (camera writes to remote)
	writeChar := primarySvc.AddCharacteristic(shortUUID(protocol.UUIDRemoteCharWrite))
	writeChar.HandleWriteFunc(func(_ context.Context, r gatt.Request, data []byte) byte {
		logger.Debugf(ctx, "received write from camera: %X", data)
		return gatt.StatusSuccess
	})

	// CE82 — Notify characteristic (remote sends commands to camera)
	notifyChar := primarySvc.AddCharacteristic(shortUUID(protocol.UUIDRemoteCharNotify))
	notifyChar.HandleNotifyFunc(func(_ context.Context, r gatt.Request, n gatt.Notifier) {
		logger.Infof(ctx, "camera subscribed to notifications")
		s.mu.Lock()
		s.notifier = n
		s.mu.Unlock()
	})

	// CE83 — Read characteristic (static value identifying remote version)
	readChar := primarySvc.AddCharacteristic(shortUUID(protocol.UUIDRemoteCharRead))
	readChar.SetValue([]byte{0x02, 0x01}) // Remote protocol version

	// Secondary D0FF service — peripheral information
	secondarySvc := gatt.NewService(fullUUID(protocol.UUIDRemoteSecondaryService))

	// FFD1 — Device name
	nameChar := secondarySvc.AddCharacteristic(fullUUID(protocol.UUIDRemoteCharDeviceName))
	nameChar.SetValue([]byte(s.name))

	// FFD2 — Firmware version
	fwChar := secondarySvc.AddCharacteristic(fullUUID(protocol.UUIDRemoteCharFirmwareVersion))
	fwChar.SetValue([]byte("1.0.0"))

	// FFD3 — Peripheral info 1
	info1Char := secondarySvc.AddCharacteristic(fullUUID(protocol.UUIDRemoteCharPeripheralInfo1))
	info1Char.SetValue([]byte{0x30, 0x1e, 0x90, 0x01})

	// FFD4 — Peripheral info 2
	info2Char := secondarySvc.AddCharacteristic(fullUUID(protocol.UUIDRemoteCharPeripheralInfo2))
	info2Char.SetValue([]byte{0x18, 0x00, 0x20, 0x01})

	if err := s.device.SetServices(ctx, []*gatt.Service{primarySvc, secondarySvc}); err != nil {
		return fmt.Errorf("failed to set services: %w", err)
	}

	return nil
}

// Advertise starts BLE advertising with the remote name.
func (s *Server) Advertise(ctx context.Context) error {
	return s.device.AdvertiseNameAndServices(ctx, s.name, []gatt.UUID{shortUUID(protocol.UUIDRemoteService)})
}

// SendCommand sends a 9-byte remote command to the connected camera
// via CE82 notifications.
func (s *Server) SendCommand(cmd protocol.RemoteCommand) error {
	s.mu.Lock()
	n := s.notifier
	s.mu.Unlock()

	if n == nil {
		return fmt.Errorf("no camera connected (notifier not set)")
	}
	if n.Done() {
		return fmt.Errorf("camera disconnected")
	}

	_, err := n.Write(cmd.Bytes())
	return err
}

// Stop stops advertising and cleans up.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	s.running = false
	s.notifier = nil
	s.mu.Unlock()

	if err := s.device.StopAdvertising(ctx); err != nil {
		return fmt.Errorf("failed to stop advertising: %w", err)
	}
	return nil
}
