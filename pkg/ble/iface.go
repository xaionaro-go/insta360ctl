package ble

import (
	"context"
	"fmt"
	"net"
)

// DeviceID is a MAC address identifying a BLE device.
type DeviceID = net.HardwareAddr

// ParseDeviceID parses a string like "AA:BB:CC:DD:EE:FF" into a DeviceID.
func ParseDeviceID(s string) (DeviceID, error) {
	hw, err := net.ParseMAC(s)
	if err != nil {
		return nil, fmt.Errorf("invalid device ID '%s': %w", s, err)
	}
	return DeviceID(hw), nil
}

// ScanResult is a discovered BLE device.
type ScanResult struct {
	Address   string
	LocalName string
	RSSI      int
}

// CharProps describes characteristic properties.
type CharProps uint8

const (
	CharPropRead     CharProps = 1 << iota
	CharPropWriteNR            // Write Without Response
	CharPropWrite              // Write With Response
	CharPropNotify
	CharPropIndicate
)

// Adapter abstracts a BLE adapter (radio).
type Adapter interface {
	// Scan starts scanning for BLE peripherals. The callback is called for each
	// discovered device. Scanning continues until ctx is cancelled or StopScan is called.
	Scan(ctx context.Context, callback func(ScanResult)) error

	// StopScan stops an ongoing scan.
	StopScan() error

	// Connect connects to a peripheral by its address string.
	Connect(ctx context.Context, addr string) (Peripheral, error)

	// Close shuts down the adapter.
	Close(ctx context.Context) error
}

// Peripheral abstracts a connected BLE peripheral.
type Peripheral interface {
	// ID returns the peripheral's address string.
	ID() string

	// DiscoverServices discovers all GATT services.
	DiscoverServices(ctx context.Context) ([]Service, error)

	// SetMTU requests a new ATT MTU.
	SetMTU(ctx context.Context, mtu uint16) error

	// Disconnect disconnects from the peripheral.
	Disconnect(ctx context.Context) error
}

// Service abstracts a GATT service.
type Service interface {
	// UUID returns the service UUID string (lowercase, with dashes).
	UUID() string

	// DiscoverCharacteristics discovers all characteristics in this service.
	DiscoverCharacteristics(ctx context.Context) ([]Characteristic, error)
}

// Characteristic abstracts a GATT characteristic.
type Characteristic interface {
	// UUID returns the characteristic UUID string (lowercase, with dashes).
	UUID() string

	// Write writes data to the characteristic.
	// If withResponse is true, waits for a write response (Write Request).
	// If false, sends without waiting (Write Command / Write Without Response).
	Write(data []byte, withResponse bool) error

	// Read reads the characteristic value.
	Read() ([]byte, error)

	// EnableNotifications subscribes to value change notifications.
	// Pass nil to disable notifications.
	EnableNotifications(callback func([]byte)) error

	// Properties returns the characteristic's properties.
	Properties() CharProps
}

// UUIDMatchesShort checks if a UUID string matches a 16-bit short UUID.
// For example, UUIDMatchesShort("0000be80-0000-1000-8000-00805f9b34fb", 0xBE80) returns true.
func UUIDMatchesShort(uuidStr string, short uint16) bool {
	expected := fmt.Sprintf("0000%04x-0000-1000-8000-00805f9b34fb", short)
	return uuidStr == expected
}

// FindCharByUUID16 searches characteristics for one matching a 16-bit UUID.
func FindCharByUUID16(chars []Characteristic, short uint16) Characteristic {
	for _, c := range chars {
		if UUIDMatchesShort(c.UUID(), short) {
			return c
		}
	}
	return nil
}

// FindServiceByUUID16 searches services for one matching a 16-bit UUID.
func FindServiceByUUID16(services []Service, short uint16) Service {
	for _, s := range services {
		if UUIDMatchesShort(s.UUID(), short) {
			return s
		}
	}
	return nil
}
