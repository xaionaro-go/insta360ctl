package protocol

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xaionaro-go/insta360ctl/pkg/protocol/messagecode"
)

func TestHeaderEncodeDecodeRoundtrip(t *testing.T) {
	tests := []struct {
		name    string
		header  Header
	}{
		{
			name: "take photo",
			header: Header{
				PayloadLength: 0,
				CommandCode:   messagecode.CodeTakePhoto,
				Sequence:      1,
			},
		},
		{
			name: "start recording with payload",
			header: Header{
				PayloadLength: 42,
				CommandCode:   messagecode.CodeStartRecording,
				Sequence:      127,
			},
		},
		{
			name: "max sequence",
			header: Header{
				PayloadLength: 100,
				CommandCode:   messagecode.CodeGetBatteryInfo,
				Sequence:      254,
			},
		},
		{
			name: "large payload",
			header: Header{
				PayloadLength: 1024,
				CommandCode:   messagecode.CodeSetGPS,
				Sequence:      50,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := tt.header.Encode()
			assert.Equal(t, HeaderSize, len(encoded))

			decoded, err := DecodeHeader(encoded[:])
			require.NoError(t, err)

			assert.Equal(t, tt.header.PayloadLength, decoded.PayloadLength)
			assert.Equal(t, tt.header.CommandCode, decoded.CommandCode)
			assert.Equal(t, tt.header.Sequence, decoded.Sequence)
		})
	}
}

func TestHeaderFixedBytes(t *testing.T) {
	h := Header{
		PayloadLength: 10,
		CommandCode:   messagecode.CodeTakePhoto,
		Sequence:      5,
	}
	buf := h.Encode()

	// Mode at offset 4 should be 0x04.
	assert.Equal(t, byte(0x04), buf[4])
	// Constant at offset 9 should be 0x02.
	assert.Equal(t, byte(0x02), buf[9])
	// Marker at offset 13 should be 0x80.
	assert.Equal(t, byte(0x80), buf[13])
	// Command code at offset 7.
	assert.Equal(t, byte(messagecode.CodeTakePhoto), buf[7])
	// Sequence at offset 10.
	assert.Equal(t, byte(5), buf[10])
}

func TestDecodeHeaderTooShort(t *testing.T) {
	_, err := DecodeHeader(make([]byte, 10))
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

func TestDecodeHeaderInvalidMode(t *testing.T) {
	buf := make([]byte, HeaderSize)
	buf[4] = 0xFF // Invalid mode
	buf[9] = 0x02
	_, err := DecodeHeader(buf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid header mode byte")
}

func TestDecodeHeaderAcceptsAnyContentType(t *testing.T) {
	// Content type byte at offset 9 is informational (0x02=protobuf, etc.)
	// and should not cause decode to fail.
	buf := make([]byte, HeaderSize)
	buf[4] = 0x04
	buf[9] = 0xFF
	hdr, err := DecodeHeader(buf)
	require.NoError(t, err)
	assert.Equal(t, messagecode.Code(0), hdr.CommandCode)
}

func TestEncodeDecodeMessage(t *testing.T) {
	payload := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	msg := EncodeMessage(messagecode.CodeSetGPS, 42, payload)

	hdr, gotPayload, err := DecodeMessage(msg)
	require.NoError(t, err)

	assert.Equal(t, messagecode.CodeSetGPS, hdr.CommandCode)
	assert.Equal(t, uint8(42), hdr.Sequence)
	assert.Equal(t, uint16(5), hdr.PayloadLength)
	assert.Equal(t, payload, gotPayload)
}

func TestDecodeMessageTruncated(t *testing.T) {
	// Valid header claiming 100 bytes of payload, but only 5 bytes after header.
	msg := EncodeMessage(messagecode.CodeTakePhoto, 1, make([]byte, 100))
	truncated := msg[:HeaderSize+5]

	_, _, err := DecodeMessage(truncated)
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

func TestGo2HeaderEncodeDecodeRoundtrip(t *testing.T) {
	tests := []struct {
		name    string
		cmd     messagecode.Code
		payload []byte
	}{
		{
			name:    "take photo no payload",
			cmd:     messagecode.CodeTakePhoto,
			payload: nil,
		},
		{
			name:    "set mode with payload",
			cmd:     messagecode.CodeSetCaptureMode,
			payload: []byte{0x01},
		},
		{
			name:    "check auth with string payload",
			cmd:     messagecode.CodeCheckAuthorization,
			payload: []byte("AA:BB:CC:DD:EE:FF"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := EncodeGo2Message(tt.cmd, 0, tt.payload)
			assert.Equal(t, HeaderSize+len(tt.payload), len(msg))

			hdr, gotPayload, err := DecodeGo2Message(msg)
			require.NoError(t, err)

			assert.Equal(t, tt.cmd, hdr.CommandCode)
			assert.Equal(t, uint16(len(tt.payload)), hdr.PayloadLength)
			if len(tt.payload) == 0 {
				assert.Empty(t, gotPayload)
			} else {
				assert.Equal(t, tt.payload, gotPayload)
			}
		})
	}
}

func TestGo2HeaderFixedBytes(t *testing.T) {
	h := Header{
		PayloadLength: 5,
		CommandCode:   messagecode.CodeTakePhoto,
	}
	buf := h.EncodeGo2()

	// Bytes 0-3: uint32 LE total_inner_size = 16 + 5 = 21.
	totalSize := uint32(buf[0]) | uint32(buf[1])<<8 | uint32(buf[2])<<16 | uint32(buf[3])<<24
	assert.Equal(t, uint32(21), totalSize)

	// Mode at offset 4 should be 0x04.
	assert.Equal(t, byte(0x04), buf[4])
	// ContentType at offset 9 should be 0x02.
	assert.Equal(t, byte(0x02), buf[9])
	// Command code at offset 7 (uint16 LE).
	assert.Equal(t, byte(messagecode.CodeTakePhoto), buf[7])
	assert.Equal(t, byte(0x00), buf[8])
	// Fragment offset = 0 at bytes 10-12.
	assert.Equal(t, byte(0x00), buf[10])
	assert.Equal(t, byte(0x00), buf[11])
	assert.Equal(t, byte(0x00), buf[12])
	// is_last flag at byte 13.
	assert.Equal(t, byte(0x80), buf[13])
}
