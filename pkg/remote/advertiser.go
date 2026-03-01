package remote

import (
	"context"
	"fmt"

	"github.com/xaionaro-go/gatt"
)

// WakeOnBLE advertises an iBeacon-format packet to wake a sleeping
// Insta360 camera. The camera listens for specific iBeacon data
// containing its serial-derived ID even when in low-power sleep.
//
// cameraID is a 6-byte identifier derived from the camera's serial number
// (typically found in camera BLE settings).
func WakeOnBLE(ctx context.Context, device gatt.Device, cameraID [6]byte) error {
	// iBeacon manufacturer data format:
	//   Bytes 0-1:  Apple company ID (0x4C, 0x00)
	//   Byte 2:     iBeacon type (0x02)
	//   Byte 3:     Data length (0x15 = 21)
	//   Bytes 4-19: Proximity UUID (16 bytes, contains ORBIT magic + camera ID)
	//   Bytes 20-21: Major (0x00, 0x01)
	//   Bytes 22-23: Minor (0x00, 0x01)
	//   Byte 24:    TX Power (-59 dBm = 0xC5)
	data := make([]byte, 25)
	data[0] = 0x4C // Apple company ID low
	data[1] = 0x00 // Apple company ID high
	data[2] = 0x02 // iBeacon type
	data[3] = 0x15 // Data length (21 bytes)

	// Proximity UUID: ORBIT magic bytes + camera ID
	// "ORBIT" prefix is the Insta360 remote wake identifier
	copy(data[4:9], []byte("ORBIT"))
	// Padding
	data[9] = 0x00
	// Camera-specific 6-byte ID
	copy(data[10:16], cameraID[:])
	// Remaining UUID bytes
	data[16] = 0x00
	data[17] = 0x00
	data[18] = 0x00
	data[19] = 0x00

	// Major
	data[20] = 0x00
	data[21] = 0x01
	// Minor
	data[22] = 0x00
	data[23] = 0x01
	// TX Power
	data[24] = 0xC5 // -59 dBm

	if err := device.AdvertiseIBeaconData(ctx, data); err != nil {
		return fmt.Errorf("failed to advertise Wake-on-BLE: %w", err)
	}

	return nil
}
