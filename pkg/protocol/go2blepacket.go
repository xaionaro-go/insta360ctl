package protocol

import (
	"encoding/binary"
	"fmt"
)

// Go2BlePacket implements the BLE transport envelope used by GO 2, GO 3, and GO 3S cameras.
//
// All BLE communication on these cameras is wrapped in this envelope format,
// which was reverse-engineered from libOne.so (Go2BlePacket.cpp).
//
// Three packet variants exist:
//
//  1. Message packet (newBlePacket): wraps a Packet (inner header + payload)
//     Byte 0:   0xFF (marker)
//     Byte 1:   0x07 (constant)
//     Byte 2:   0x40 (message type)
//     Bytes 3-4: uint16 LE inner data size
//     Bytes 5..N: inner data (Packet header + message payload)
//     Last 2:   CRC-16/Modbus over all preceding bytes
//
//  2. Sync packet (newBleSyncPacket): establishes bidirectional communication
//     Same as message packet but Byte 2 = 0x41
//
//  3. Simplified packet (newBlePacket2): used for wake-up authorization etc.
//     Byte 0:   0xFF (marker)
//     Byte 1:   type (e.g. 0x0C for wake-up auth)
//     Byte 2:   command
//     Bytes 3-4: uint16 LE data size
//     Bytes 5..N: data
//     Last 2:   CRC-16/Modbus over all preceding bytes
//
// Constants derived from Ghidra decompilation:
//   - getBlePacketHeadSize() = 5
//   - getMaxMessageContentSize() = 0x1EF - getMessageHeadSize() = 495 - 9 = 486
//   - CRC polynomial: 0xA001, init: 0xFFFF (CRC-16/Modbus)

const (
	// Go2BLEHeaderSize is the BLE envelope header size (5 bytes).
	Go2BLEHeaderSize = 5

	// Go2BLECRCSize is the trailing CRC size (2 bytes).
	Go2BLECRCSize = 2

	// Go2BLEOverhead is total envelope overhead: header + CRC.
	Go2BLEOverhead = Go2BLEHeaderSize + Go2BLECRCSize // 7

	// Go2BLEMarker is the first byte of every Go2BlePacket.
	Go2BLEMarker = 0xFF

	// Go2BLETypeMessage is byte 1 for outgoing message packets (app → camera).
	Go2BLETypeMessage = 0x07

	// Go2BLETypeResponse is byte 1 for incoming response packets (camera → app).
	Go2BLETypeResponse = 0x06

	// Go2BLESubtypeMessage is byte 2 for message packets.
	Go2BLESubtypeMessage = 0x40

	// Go2BLESubtypeSync is byte 2 for sync packets.
	Go2BLESubtypeSync = 0x41

	// Go2BLEMaxInnerSize is the maximum inner data size (from 0x1EF = 495).
	Go2BLEMaxInnerSize = 0x1EF
)

// EncodeGo2BleMessagePacket wraps inner packet data in a Go2BlePacket message envelope.
//
// Wire format: [0xFF, 0x07, 0x40, size_lo, size_hi] + innerData + CRC16
//
// The innerData should be a complete inner packet (16-byte header + payload),
// as produced by EncodeGo2InnerMessage().
func EncodeGo2BleMessagePacket(innerData []byte) []byte {
	return encodeGo2BlePacket(Go2BLETypeMessage, Go2BLESubtypeMessage, innerData)
}

// EncodeGo2BleSyncPacket wraps sync data in a Go2BlePacket sync envelope.
//
// Wire format: [0xFF, 0x07, 0x41, size_lo, size_hi] + syncData + CRC16
func EncodeGo2BleSyncPacket(syncData []byte) []byte {
	return encodeGo2BlePacket(Go2BLETypeMessage, Go2BLESubtypeSync, syncData)
}

// EncodeGo2BlePacket2 creates a simplified Go2BlePacket (used for wake-up auth, etc.).
//
// Wire format: [0xFF, msgType, cmd, size_lo, size_hi] + params + CRC16
//
// Example: sendWakeUpAuthorization uses msgType=0x0C, cmd=0x02, params=auth_id_bytes
func EncodeGo2BlePacket2(msgType, cmd byte, params []byte) []byte {
	return encodeGo2BlePacket(msgType, cmd, params)
}

// encodeGo2BlePacket is the common encoder for all Go2BlePacket variants.
func encodeGo2BlePacket(typeByte, subtypeByte byte, data []byte) []byte {
	dataLen := len(data)
	totalLen := Go2BLEHeaderSize + dataLen + Go2BLECRCSize

	buf := make([]byte, totalLen)

	// BLE header (5 bytes).
	buf[0] = Go2BLEMarker
	buf[1] = typeByte
	buf[2] = subtypeByte
	binary.LittleEndian.PutUint16(buf[3:5], uint16(dataLen))

	// Inner data.
	copy(buf[Go2BLEHeaderSize:], data)

	// CRC-16/Modbus over header + data (everything except CRC itself).
	crcOffset := Go2BLEHeaderSize + dataLen
	crc := CRC16Modbus(buf[:crcOffset])
	binary.LittleEndian.PutUint16(buf[crcOffset:], crc)

	return buf
}

// DecodeGo2BlePacket strips the Go2BlePacket envelope and verifies CRC.
//
// Returns the inner data, the packet subtype (0x40=message, 0x41=sync),
// and the type byte (0x07 for standard packets).
func DecodeGo2BlePacket(b []byte) (innerData []byte, typeByte byte, subtypeByte byte, err error) {
	if len(b) < Go2BLEOverhead {
		return nil, 0, 0, fmt.Errorf("Go2BlePacket too short: %d bytes (min %d)", len(b), Go2BLEOverhead)
	}

	if b[0] != Go2BLEMarker {
		return nil, 0, 0, fmt.Errorf("Go2BlePacket missing marker: expected 0x%02X, got 0x%02X", Go2BLEMarker, b[0])
	}

	typeByte = b[1]
	subtypeByte = b[2]

	// Read declared inner data size from bytes 3-4.
	declaredSize := binary.LittleEndian.Uint16(b[3:5])
	expectedTotal := Go2BLEHeaderSize + int(declaredSize) + Go2BLECRCSize

	if len(b) < expectedTotal {
		return nil, 0, 0, fmt.Errorf("Go2BlePacket declares %d inner bytes but packet is only %d bytes (need %d)",
			declaredSize, len(b), expectedTotal)
	}

	// Verify CRC over header + inner data.
	crcOffset := Go2BLEHeaderSize + int(declaredSize)
	expectedCRC := binary.LittleEndian.Uint16(b[crcOffset:])
	actualCRC := CRC16Modbus(b[:crcOffset])
	if expectedCRC != actualCRC {
		return nil, 0, 0, fmt.Errorf("Go2BlePacket CRC mismatch: expected 0x%04X, got 0x%04X", expectedCRC, actualCRC)
	}

	innerData = b[Go2BLEHeaderSize:crcOffset]
	return innerData, typeByte, subtypeByte, nil
}

// Go2BleProbePacketSize reads the declared inner data size from bytes 3-4.
// This matches Go2BlePacket::probePacketSize() from libOne.so.
func Go2BleProbePacketSize(b []byte) (uint16, error) {
	if len(b) < 4 {
		return 0, fmt.Errorf("Go2BlePacket too short to probe size: %d bytes (need 4)", len(b))
	}
	return binary.LittleEndian.Uint16(b[3:5]), nil
}
