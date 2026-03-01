package protocol

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/xaionaro-go/insta360ctl/pkg/protocol/messagecode"
)

const (
	// HeaderSize is the size of an Architecture B message header.
	// This is getPacketHeadSize() (7) + getMessageHeadSize() (9) = 16 bytes.
	HeaderSize = 16

	// headerMode is the mode byte at offset 4 (0x04 = message type).
	headerMode = 0x04

	// ContentTypeProtobuf is the content type byte at offset 9 (0x02 = protobuf).
	ContentTypeProtobuf = 0x02

	// headerIsLast is the is_last_fragment flag at offset 13 (bit 7 = 0x80).
	// For single-fragment messages this is always set.
	headerIsLast = 0x80
)

// Header represents a decoded Architecture B message header.
//
// Two wire formats exist that share the same logical structure:
//
// Header16 format (X3, ONE R, ONE RS — sent directly on BLE):
//
//	Offset  Size  Description
//	0       2     uint16 LE: payload_length (excludes header)
//	2       2     Reserved (zero)
//	4       1     Mode (0x04)
//	5       2     Reserved (zero)
//	7       1     Command code (messagecode.Code)
//	8       1     Reserved (zero)
//	9       1     ContentType (0x02 = protobuf)
//	10      1     Sequence number (1-254)
//	11      2     Reserved (zero)
//	13      1     is_last (0x80)
//	14      2     Reserved (zero)
//
// Go2 inner format (GO 2/GO 3 — wrapped in Go2BlePacket envelope):
//
//	Offset  Size  Description
//	0       4     uint32 LE: total_inner_size (= HeaderSize + payload_length)
//	4       1     Mode (0x04)
//	5       1     Alignment flag
//	6       1     Reserved
//	7       2     uint16 LE: MessageCode
//	9       1     ContentType (0x02 = protobuf)
//	10      4     Packed: fragment_offset(30b) | direction(1b) | is_last(1b)
//	14      2     Reserved (zero)
type Header struct {
	PayloadLength uint16
	CommandCode   messagecode.Code
	Sequence      uint8
}

// Encode serializes the header into a 16-byte buffer (Header16 format for X3/ONE R).
func (h *Header) Encode() [HeaderSize]byte {
	var buf [HeaderSize]byte

	binary.LittleEndian.PutUint16(buf[0:2], h.PayloadLength)
	// buf[2:4] = 0 (reserved)
	buf[4] = headerMode
	// buf[5:7] = 0 (reserved)
	buf[7] = byte(h.CommandCode)
	// buf[8] = 0 (reserved)
	buf[9] = ContentTypeProtobuf
	buf[10] = h.Sequence
	// buf[11:13] = 0 (reserved)
	buf[13] = headerIsLast
	// buf[14:16] = 0 (reserved)

	return buf
}

// EncodeGo2 serializes the header in the Go2BlePacket inner format.
// Key differences from Encode():
//   - bytes 0-3: uint32 total_inner_size (not uint16 payload_length)
//   - bytes 7-8: uint16 MessageCode (not single byte)
//   - byte 10: sequence number (used by GO 3 for request/response matching)
//   - byte 13: is_last flag (0x80)
func (h *Header) EncodeGo2() [HeaderSize]byte {
	var buf [HeaderSize]byte

	// Bytes 0-3: uint32 LE total inner size (header + payload).
	totalInnerSize := uint32(HeaderSize) + uint32(h.PayloadLength)
	binary.LittleEndian.PutUint32(buf[0:4], totalInnerSize)

	// Byte 4: mode = 0x04 (message type).
	buf[4] = headerMode

	// Bytes 7-8: uint16 LE MessageCode.
	binary.LittleEndian.PutUint16(buf[7:9], uint16(h.CommandCode))

	// Byte 9: ContentType = 0x02 (protobuf).
	buf[9] = ContentTypeProtobuf

	// Byte 10: sequence number for request/response matching.
	buf[10] = h.Sequence

	// Byte 13: is_last flag.
	buf[13] = headerIsLast

	return buf
}

// DecodeHeader parses a 16-byte Header16 format header from a byte slice.
// Use this for X3/ONE R/ONE RS messages.
func DecodeHeader(b []byte) (*Header, error) {
	if len(b) < HeaderSize {
		return nil, fmt.Errorf("%w: need %d bytes for header, got %d", io.ErrUnexpectedEOF, HeaderSize, len(b))
	}

	if b[4] != headerMode {
		return nil, fmt.Errorf("invalid header mode byte: expected 0x%02X, got 0x%02X", headerMode, b[4])
	}

	return &Header{
		PayloadLength: binary.LittleEndian.Uint16(b[0:2]),
		CommandCode:   messagecode.Code(uint16(b[7])),
		Sequence:      b[10],
	}, nil
}

// DecodeGo2Header parses a 16-byte Go2 format inner header from a byte slice.
// Use this for GO 2/GO 3 messages (after stripping Go2BlePacket envelope).
func DecodeGo2Header(b []byte) (*Header, error) {
	if len(b) < HeaderSize {
		return nil, fmt.Errorf("%w: need %d bytes for header, got %d", io.ErrUnexpectedEOF, HeaderSize, len(b))
	}

	if b[4] != headerMode {
		return nil, fmt.Errorf("invalid header mode byte: expected 0x%02X, got 0x%02X", headerMode, b[4])
	}

	// Bytes 0-3: uint32 total_inner_size.
	totalSize := binary.LittleEndian.Uint32(b[0:4])
	var payloadLength uint16
	if totalSize >= HeaderSize {
		payloadLength = uint16(totalSize - HeaderSize)
	}

	// Bytes 7-8: uint16 MessageCode.
	cmdCode := binary.LittleEndian.Uint16(b[7:9])

	return &Header{
		PayloadLength: payloadLength,
		CommandCode:   messagecode.Code(cmdCode),
		Sequence:      b[10],
	}, nil
}

// EncodeMessage creates a complete Architecture B message with header + payload.
// Uses the Header16 format (for X3/ONE R/ONE RS).
func EncodeMessage(cmd messagecode.Code, seq uint8, payload []byte) []byte {
	h := Header{
		PayloadLength: uint16(len(payload)),
		CommandCode:   cmd,
		Sequence:      seq,
	}
	hdr := h.Encode()
	msg := make([]byte, HeaderSize+len(payload))
	copy(msg[:HeaderSize], hdr[:])
	copy(msg[HeaderSize:], payload)
	return msg
}

// EncodeGo2Message creates a complete inner message for Go2BlePacket wrapping.
// The seq parameter is the sequence number for request/response matching.
func EncodeGo2Message(cmd messagecode.Code, seq uint8, payload []byte) []byte {
	h := Header{
		PayloadLength: uint16(len(payload)),
		CommandCode:   cmd,
		Sequence:      seq,
	}
	hdr := h.EncodeGo2()
	msg := make([]byte, HeaderSize+len(payload))
	copy(msg[:HeaderSize], hdr[:])
	copy(msg[HeaderSize:], payload)
	return msg
}

// DecodeMessage parses a complete Header16 format message into header and payload.
func DecodeMessage(b []byte) (*Header, []byte, error) {
	hdr, err := DecodeHeader(b)
	if err != nil {
		return nil, nil, err
	}

	totalLen := HeaderSize + int(hdr.PayloadLength)
	if len(b) < totalLen {
		return nil, nil, fmt.Errorf("%w: message declares %d payload bytes, but only %d available",
			io.ErrUnexpectedEOF, hdr.PayloadLength, len(b)-HeaderSize)
	}

	payload := b[HeaderSize:totalLen]
	return hdr, payload, nil
}

// DecodeGo2Message parses a complete Go2 format inner message into header and payload.
func DecodeGo2Message(b []byte) (*Header, []byte, error) {
	hdr, err := DecodeGo2Header(b)
	if err != nil {
		return nil, nil, err
	}

	totalLen := HeaderSize + int(hdr.PayloadLength)
	if len(b) < totalLen {
		return nil, nil, fmt.Errorf("%w: message declares %d payload bytes, but only %d available",
			io.ErrUnexpectedEOF, hdr.PayloadLength, len(b)-HeaderSize)
	}

	payload := b[HeaderSize:totalLen]
	return hdr, payload, nil
}
