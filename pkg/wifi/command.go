package wifi

import (
	"encoding/binary"
	"fmt"
	"sync/atomic"

	"google.golang.org/protobuf/proto"
)

// WiFi command codes (different numbering from BLE Architecture B).
const (
	CmdBegin              uint16 = 0
	CmdStartLiveStream    uint16 = 1
	CmdStopLiveStream     uint16 = 2
	CmdTakePicture        uint16 = 3
	CmdStartCapture       uint16 = 4
	CmdStopCapture        uint16 = 5
	CmdCancelCapture      uint16 = 6
	CmdSetOptions         uint16 = 7
	CmdGetOptions         uint16 = 8
	CmdGetFileList              uint16 = 13
	CmdGetCaptureStatus         uint16 = 15
	CmdSetWifiConnectionInfo    uint16 = 112
	CmdGetWifiConnectionInfo    uint16 = 113
)

// WiFi response/notification codes.
const (
	RespOK                        uint16 = 200
	RespError                     uint16 = 500
	NotifyBatteryLow              uint16 = 8196
	NotifyStorageUpdate           uint16 = 8198
	NotifyStorageFull             uint16 = 8199
	NotifyCaptureStopped          uint16 = 8201
	NotifyCurrentCaptureStatus    uint16 = 8208
	NotifyWifiConnectionResult    uint16 = 8232
)

// commandHeaderSize is the fixed size of the command/response header.
const commandHeaderSize = 12

var globalSeq atomic.Uint32

func init() {
	globalSeq.Store(1)
}

func nextSeq() uint32 {
	return globalSeq.Add(1)
}

// buildCommandPacket creates a WiFi protocol command packet.
//
// Header format (12 bytes):
//
//	Offset 0-2:  packet type (0x04, 0x00, 0x00)
//	Offset 3-4:  message code (uint16 LE)
//	Offset 5:    0x02 (constant)
//	Offset 6-8:  sequence number (uint24 LE)
//	Offset 9:    0x80 (constant)
//	Offset 10-11: 0x00, 0x00 (reserved)
func buildCommandPacket(cmdCode uint16, seq uint32, body []byte) []byte {
	pkt := make([]byte, commandHeaderSize+len(body))
	pkt[0] = pktTypeMessage
	pkt[1] = 0x00
	pkt[2] = 0x00
	binary.LittleEndian.PutUint16(pkt[3:5], cmdCode)
	pkt[5] = 0x02
	pkt[6] = byte(seq)
	pkt[7] = byte(seq >> 8)
	pkt[8] = byte(seq >> 16)
	pkt[9] = 0x80
	pkt[10] = 0x00
	pkt[11] = 0x00
	copy(pkt[commandHeaderSize:], body)
	return pkt
}

// SendCommand sends a protobuf command and returns the sequence number.
func (c *Conn) SendCommand(cmdCode uint16, msg proto.Message) (uint32, error) {
	var body []byte
	if msg != nil {
		var err error
		body, err = proto.Marshal(msg)
		if err != nil {
			return 0, fmt.Errorf("marshal protobuf: %w", err)
		}
	}

	seq := nextSeq()
	pkt := buildCommandPacket(cmdCode, seq, body)

	if err := c.writePacket(pkt); err != nil {
		return 0, fmt.Errorf("send command 0x%04X: %w", cmdCode, err)
	}

	return seq, nil
}

// ParsedResponse holds a parsed WiFi protocol response.
type ParsedResponse struct {
	ResponseCode uint16
	Sequence     uint32
	Body         []byte // protobuf bytes after the 12-byte header
}

// parseMessagePayload parses a MESSAGE type payload into a ParsedResponse.
func parseMessagePayload(payload []byte) (*ParsedResponse, error) {
	if len(payload) < commandHeaderSize {
		return nil, fmt.Errorf("message payload too short: %d < %d", len(payload), commandHeaderSize)
	}

	resp := &ParsedResponse{
		ResponseCode: binary.LittleEndian.Uint16(payload[3:5]),
		Sequence:     uint32(payload[6]) | uint32(payload[7])<<8 | uint32(payload[8])<<16,
	}

	if len(payload) > commandHeaderSize {
		resp.Body = payload[commandHeaderSize:]
	}

	return resp, nil
}

// IsOK returns true if the response code indicates success.
func (r *ParsedResponse) IsOK() bool {
	return r.ResponseCode == RespOK
}

// IsError returns true if the response code indicates an error.
func (r *ParsedResponse) IsError() bool {
	return r.ResponseCode == RespError
}

// IsNotification returns true if this is a camera notification.
func (r *ParsedResponse) IsNotification() bool {
	return r.ResponseCode >= 8000
}
