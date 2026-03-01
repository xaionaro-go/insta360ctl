package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xaionaro-go/insta360ctl/pkg/protocol/messagecode"
)

func TestEncodeDecodeGo2BleMessagePacket(t *testing.T) {
	// Build an inner message.
	innerMsg := EncodeGo2Message(messagecode.CodeTakePhoto, 0, nil)

	// Wrap in Go2BlePacket.
	packet := EncodeGo2BleMessagePacket(innerMsg)

	// Verify envelope structure.
	assert.Equal(t, byte(Go2BLEMarker), packet[0])       // 0xFF
	assert.Equal(t, byte(Go2BLETypeMessage), packet[1])   // 0x07
	assert.Equal(t, byte(Go2BLESubtypeMessage), packet[2]) // 0x40

	// Decode it back.
	innerData, typeByte, subtypeByte, err := DecodeGo2BlePacket(packet)
	require.NoError(t, err)
	assert.Equal(t, byte(Go2BLETypeMessage), typeByte)
	assert.Equal(t, byte(Go2BLESubtypeMessage), subtypeByte)
	assert.Equal(t, innerMsg, innerData)

	// Parse the inner header using Go2 format.
	hdr, _, err := DecodeGo2Message(innerData)
	require.NoError(t, err)
	assert.Equal(t, messagecode.CodeTakePhoto, hdr.CommandCode)
	assert.Equal(t, uint16(0), hdr.PayloadLength)
}

func TestEncodeDecodeGo2BleSyncPacket(t *testing.T) {
	syncData := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	packet := EncodeGo2BleSyncPacket(syncData)

	assert.Equal(t, byte(Go2BLEMarker), packet[0])
	assert.Equal(t, byte(Go2BLETypeMessage), packet[1])
	assert.Equal(t, byte(Go2BLESubtypeSync), packet[2]) // 0x41

	innerData, typeByte, subtypeByte, err := DecodeGo2BlePacket(packet)
	require.NoError(t, err)
	assert.Equal(t, byte(Go2BLETypeMessage), typeByte)
	assert.Equal(t, byte(Go2BLESubtypeSync), subtypeByte)
	assert.Equal(t, syncData, innerData)
}

func TestEncodeDecodeGo2BlePacket2(t *testing.T) {
	// Wake-up authorization: type=0x0C, cmd=0x02, params=auth_id
	authID := []byte("ABCDEF")
	packet := EncodeGo2BlePacket2(0x0C, 0x02, authID)

	assert.Equal(t, byte(Go2BLEMarker), packet[0])
	assert.Equal(t, byte(0x0C), packet[1])
	assert.Equal(t, byte(0x02), packet[2])

	innerData, typeByte, subtypeByte, err := DecodeGo2BlePacket(packet)
	require.NoError(t, err)
	assert.Equal(t, byte(0x0C), typeByte)
	assert.Equal(t, byte(0x02), subtypeByte)
	assert.Equal(t, authID, innerData)
}

func TestGo2BlePacketCRCValidation(t *testing.T) {
	packet := EncodeGo2BleMessagePacket([]byte{0x01, 0x02, 0x03})

	// Corrupt the CRC.
	packet[len(packet)-1] ^= 0xFF

	_, _, _, err := DecodeGo2BlePacket(packet)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CRC mismatch")
}

func TestGo2BlePacketTooShort(t *testing.T) {
	_, _, _, err := DecodeGo2BlePacket([]byte{0xFF, 0x07})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

func TestGo2BlePacketBadMarker(t *testing.T) {
	packet := EncodeGo2BleMessagePacket([]byte{0x01})
	packet[0] = 0xFE // corrupt marker

	_, _, _, err := DecodeGo2BlePacket(packet)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing marker")
}

func TestGo2BleProbePacketSize(t *testing.T) {
	innerData := make([]byte, 42)
	packet := EncodeGo2BleMessagePacket(innerData)

	size, err := Go2BleProbePacketSize(packet)
	require.NoError(t, err)
	assert.Equal(t, uint16(42), size)
}

func TestGo2BleProbePacketSizeTooShort(t *testing.T) {
	_, err := Go2BleProbePacketSize([]byte{0xFF, 0x07, 0x40})
	assert.Error(t, err)
}

func TestGo2BlePacketFullRoundtrip(t *testing.T) {
	// Full roundtrip: create command → wrap → decode → verify.
	payload := []byte{0x01, 0x02, 0x03, 0x04}
	innerMsg := EncodeGo2Message(messagecode.CodeGetBatteryInfo, 1, payload)
	packet := EncodeGo2BleMessagePacket(innerMsg)

	// Decode Go2BlePacket envelope.
	innerData, _, subtype, err := DecodeGo2BlePacket(packet)
	require.NoError(t, err)
	assert.Equal(t, byte(Go2BLESubtypeMessage), subtype)

	// Decode inner message using Go2 format.
	hdr, gotPayload, err := DecodeGo2Message(innerData)
	require.NoError(t, err)
	assert.Equal(t, messagecode.CodeGetBatteryInfo, hdr.CommandCode)
	assert.Equal(t, uint16(4), hdr.PayloadLength)
	assert.Equal(t, payload, gotPayload)
}
