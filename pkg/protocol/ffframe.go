package protocol

import (
	"encoding/binary"
	"fmt"
)

// FF-framed protocol format used by GO 2, GO 3, GO 3S.
//
// Layout:
//
//	Byte 0:     0xFF (frame marker)
//	Byte 1:     Message type/identifier
//	Byte 2:     Command code
//	Bytes 3..N: Parameters (variable length)
//	Bytes ...:  0xAB 0xBA padding (fills to fixed buffer size)
//	Last 2:     CRC-16/Modbus (little-endian) over all preceding bytes
//
// The padding brings the total pre-CRC data to a fixed size (typically 15 bytes
// for a total of 17 bytes including CRC).

const (
	// FFFrameMarker is the first byte of every FF-framed message.
	FFFrameMarker = 0xFF

	// FFFrameMinSize is the minimum valid FF-frame: marker + type + cmd + CRC(2).
	FFFrameMinSize = 5

	// FFFrameDefaultBufSize is the default total message size (15 data + 2 CRC).
	FFFrameDefaultBufSize = 17

	// FFFramePaddingByte1 and FFFramePaddingByte2 form the ABBA padding pattern.
	FFFramePaddingByte1 = 0xAB
	FFFramePaddingByte2 = 0xBA
)

// FFFrame represents a decoded FF-framed message.
type FFFrame struct {
	MsgType byte   // Byte 1: message type/identifier
	Command byte   // Byte 2: command code
	Params  []byte // Bytes 3+: parameters (without padding)
}

// EncodeFFFrame creates an FF-framed message with CRC.
// The bufSize parameter controls the total output size (including CRC).
// If bufSize is 0, the message is not padded (minimum size).
func EncodeFFFrame(msgType, command byte, params []byte, bufSize int) []byte {
	// Header: FF + msgType + command + params
	headerLen := 3 + len(params)

	if bufSize == 0 {
		bufSize = headerLen + 2 // no padding, just CRC
	}

	if bufSize < headerLen+2 {
		bufSize = headerLen + 2
	}

	msg := make([]byte, bufSize)
	msg[0] = FFFrameMarker
	msg[1] = msgType
	msg[2] = command
	copy(msg[3:], params)

	// Fill padding with ABBA pattern.
	dataEnd := bufSize - 2 // last 2 bytes are CRC
	for i := headerLen; i < dataEnd; i++ {
		if (i-headerLen)%2 == 0 {
			msg[i] = FFFramePaddingByte1
		} else {
			msg[i] = FFFramePaddingByte2
		}
	}

	// Compute and append CRC-16/Modbus (little-endian).
	crc := CRC16Modbus(msg[:dataEnd])
	binary.LittleEndian.PutUint16(msg[dataEnd:], crc)

	return msg
}

// DecodeFFFrame parses an FF-framed message, verifying the CRC.
// Returns the decoded frame and the raw data (excluding CRC) for further analysis.
func DecodeFFFrame(b []byte) (*FFFrame, error) {
	if len(b) < FFFrameMinSize {
		return nil, fmt.Errorf("FF-frame too short: %d bytes (min %d)", len(b), FFFrameMinSize)
	}

	if b[0] != FFFrameMarker {
		return nil, fmt.Errorf("FF-frame missing marker: expected 0x%02X, got 0x%02X", FFFrameMarker, b[0])
	}

	// Verify CRC.
	dataLen := len(b) - 2
	expectedCRC := binary.LittleEndian.Uint16(b[dataLen:])
	actualCRC := CRC16Modbus(b[:dataLen])
	if expectedCRC != actualCRC {
		return nil, fmt.Errorf("FF-frame CRC mismatch: expected 0x%04X, got 0x%04X", expectedCRC, actualCRC)
	}

	// Strip ABBA padding from the end of the data portion.
	params := b[3:dataLen]
	params = stripABBAPadding(params)

	return &FFFrame{
		MsgType: b[1],
		Command: b[2],
		Params:  params,
	}, nil
}

// stripABBAPadding removes trailing 0xAB 0xBA padding bytes.
func stripABBAPadding(b []byte) []byte {
	end := len(b)
	for end >= 2 && b[end-2] == FFFramePaddingByte1 && b[end-1] == FFFramePaddingByte2 {
		end -= 2
	}
	// Handle a trailing lone 0xAB.
	if end >= 1 && b[end-1] == FFFramePaddingByte1 {
		end--
	}
	return b[:end]
}
