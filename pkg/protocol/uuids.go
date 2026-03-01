package protocol

// UUIDs are defined as uint16 short IDs for standard Bluetooth SIG services.
// Consumers convert to native UUID types using their BLE library.
//
// The full 128-bit UUID for a 16-bit short ID is:
//   0000XXXX-0000-1000-8000-00805F9B34FB

// Architecture A: GPS Remote emulation
const (
	UUIDRemoteService          uint16 = 0xCE80
	UUIDRemoteCharWrite        uint16 = 0xCE81 // Camera → Remote data
	UUIDRemoteCharNotify       uint16 = 0xCE82 // Remote → Camera commands
	UUIDRemoteCharRead         uint16 = 0xCE83 // Static: 0x02 0x01
)

// Architecture B: Direct camera control
const (
	UUIDDirectService          uint16 = 0xBE80
	UUIDDirectCharWrite        uint16 = 0xBE81 // App → Camera commands
	UUIDDirectCharNotify       uint16 = 0xBE82 // Camera → App responses
)

// B0xx: Secondary direct control service (GO 3)
const (
	UUIDDirectSecService       uint16 = 0xB000
	UUIDDirectSecCharWrite     uint16 = 0xB001
	UUIDDirectSecCharNotify1   uint16 = 0xB002
	UUIDDirectSecCharNotify2   uint16 = 0xB003
	UUIDDirectSecCharNotify3   uint16 = 0xB004
)

// AE00: Camera BLE service (from protocol analysis)
const (
	UUIDAEService              uint16 = 0xAE00
	UUIDAECharWrite            uint16 = 0xAE01
	UUIDAECharNotify           uint16 = 0xAE02
)

// Standard BLE services
const (
	UUIDGenericAccess          uint16 = 0x1800
	UUIDGATT                   uint16 = 0x1801
	UUIDDeviceInfo             uint16 = 0x180A
	UUIDBattery                uint16 = 0x180F
	UUIDBatteryLevel           uint16 = 0x2A19
)

// D0FF secondary service uses a custom (non-SIG) 128-bit UUID.
const (
	UUIDRemoteSecondaryService    = "0000d0ff-3c17-d293-8e48-14fe2e4da212"
	UUIDRemoteCharDeviceName      = "0000ffd1-3c17-d293-8e48-14fe2e4da212"
	UUIDRemoteCharFirmwareVersion = "0000ffd2-3c17-d293-8e48-14fe2e4da212"
	UUIDRemoteCharPeripheralInfo1 = "0000ffd3-3c17-d293-8e48-14fe2e4da212"
	UUIDRemoteCharPeripheralInfo2 = "0000ffd4-3c17-d293-8e48-14fe2e4da212"
)
