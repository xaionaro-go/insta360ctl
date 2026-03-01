package wifi

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/facebookincubator/go-belt/tool/logger"
	pb "github.com/xaionaro-go/insta360ctl/pkg/protocol/protobuf"
)

// StreamOptions configures the live preview stream.
type StreamOptions struct {
	// Resolution for the primary stream (default: RES_1440_720P30).
	Resolution pb.VideoResolution
	// SecondaryResolution for a secondary low-res stream (default: RES_424_240P15).
	SecondaryResolution pb.VideoResolution
	// VideoBitrate in Mbps (default: 40).
	VideoBitrate uint32
	// EnableGyro enables gyro/IMU data in the stream.
	EnableGyro bool
	// EnableAudio enables audio data in the stream.
	EnableAudio bool
}

// DefaultStreamOptions returns sensible defaults for preview streaming.
func DefaultStreamOptions() StreamOptions {
	return StreamOptions{
		Resolution:          pb.VideoResolution_RES_1440_720P30,
		SecondaryResolution: pb.VideoResolution_RES_424_240P15,
		VideoBitrate:        40,
		EnableGyro:          false,
		EnableAudio:         false,
	}
}

// StreamPacket represents a received stream data packet.
type StreamPacket struct {
	Type      byte   // StreamTypeVideo, StreamTypeGyro, or StreamTypeSync
	Timestamp uint64 // camera timestamp in microseconds
	Data      []byte // raw stream payload
}

// streamHeaderSize is the fixed size of the stream data header
// (3 bytes type + 1 byte stream_type + 8 bytes timestamp = 12).
const streamHeaderSize = 12

// Streamer manages a live preview stream session.
type Streamer struct {
	conn *Conn

	// Output channels for demuxed stream data.
	VideoCh chan StreamPacket
	GyroCh  chan StreamPacket
	SyncCh  chan StreamPacket

	// ResponseCh receives command responses during streaming.
	ResponseCh chan *ParsedResponse
}

// NewStreamer creates a Streamer wrapping an existing connection.
func NewStreamer(conn *Conn) *Streamer {
	return &Streamer{
		conn:       conn,
		VideoCh:    make(chan StreamPacket, 64),
		GyroCh:     make(chan StreamPacket, 16),
		SyncCh:     make(chan StreamPacket, 4),
		ResponseCh: make(chan *ParsedResponse, 16),
	}
}

// Start sends the START_LIVE_STREAM command and begins the receive loop.
func (s *Streamer) Start(ctx context.Context, opts StreamOptions) error {
	msg := &pb.StartLiveStream{
		EnableVideo:      true,
		EnableAudio:      opts.EnableAudio,
		VideoBitrate:     opts.VideoBitrate,
		Resolution:       opts.Resolution,
		EnableGyro:       opts.EnableGyro,
		VideoBitrate1:    opts.VideoBitrate,
		Resolution1:      opts.SecondaryResolution,
		PreviewStreamNum: 1,
	}

	seq, err := s.conn.SendCommand(CmdStartLiveStream, msg)
	if err != nil {
		return fmt.Errorf("send START_LIVE_STREAM: %w", err)
	}

	logger.Infof(ctx, "START_LIVE_STREAM sent (seq=%d), waiting for response...", seq)

	// Wait for the response before starting the read loop.
	resp, err := s.waitResponse(ctx, seq, 5*time.Second)
	if err != nil {
		return fmt.Errorf("START_LIVE_STREAM response: %w", err)
	}

	if !resp.IsOK() {
		return fmt.Errorf("START_LIVE_STREAM failed: response code %d", resp.ResponseCode)
	}

	logger.Infof(ctx, "stream started, receiving data...")

	go s.readLoop(ctx)
	return nil
}

// Stop sends the STOP_LIVE_STREAM command.
func (s *Streamer) Stop(ctx context.Context) error {
	_, err := s.conn.SendCommand(CmdStopLiveStream, &pb.StopLiveStream{})
	return err
}

// waitResponse reads packets until it finds a MESSAGE response matching the
// given sequence number, or the timeout/context expires.
func (s *Streamer) waitResponse(ctx context.Context, seq uint32, timeout time.Duration) (*ParsedResponse, error) {
	deadline := time.Now().Add(timeout)
	if err := s.conn.SetReadDeadline(deadline); err != nil {
		return nil, fmt.Errorf("set read deadline: %w", err)
	}
	defer func() { _ = s.conn.SetReadDeadline(time.Time{}) }()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		payload, err := s.conn.ReadRaw()
		if err != nil {
			return nil, err
		}

		if len(payload) < 3 {
			continue
		}

		switch payload[0] {
		case pktTypeMessage:
			resp, err := parseMessagePayload(payload)
			if err != nil {
				logger.Warnf(ctx, "failed to parse message: %v", err)
				continue
			}
			if resp.Sequence == seq {
				return resp, nil
			}
			// Not our response — queue it.
			select {
			case s.ResponseCh <- resp:
			default:
			}

		case pktTypeKeepalive:
			// Ignore.

		case pktTypeSync:
			// Ignore sync echo during wait.

		case pktTypeStream:
			// Stream data arrived before we got the response — dispatch it.
			s.dispatchStream(ctx, payload)
		}
	}
}

// readLoop continuously reads and demultiplexes incoming packets.
func (s *Streamer) readLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		payload, err := s.conn.ReadRaw()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Warnf(ctx, "read error: %v", err)
			return
		}

		if len(payload) < 3 {
			continue
		}

		switch payload[0] {
		case pktTypeStream:
			s.dispatchStream(ctx, payload)

		case pktTypeMessage:
			resp, err := parseMessagePayload(payload)
			if err != nil {
				logger.Warnf(ctx, "failed to parse message: %v", err)
				continue
			}
			select {
			case s.ResponseCh <- resp:
			default:
				logger.Debugf(ctx, "response channel full, dropping response code=%d seq=%d",
					resp.ResponseCode, resp.Sequence)
			}

		case pktTypeKeepalive:
			// Keep-alive received — connection is alive.

		case pktTypeSync:
			// Unexpected sync during streaming.
			logger.Debugf(ctx, "received sync packet during streaming")
		}
	}
}

// dispatchStream parses a stream packet and sends it to the appropriate channel.
func (s *Streamer) dispatchStream(ctx context.Context, payload []byte) {
	if len(payload) < streamHeaderSize {
		return
	}

	pkt := StreamPacket{
		Type:      payload[3],
		Timestamp: binary.LittleEndian.Uint64(payload[4:12]),
	}

	if len(payload) > streamHeaderSize {
		pkt.Data = payload[streamHeaderSize:]
	}

	switch pkt.Type {
	case StreamTypeVideo:
		select {
		case s.VideoCh <- pkt:
		default:
			// Drop if channel full — video is time-sensitive.
		}

	case StreamTypeGyro:
		select {
		case s.GyroCh <- pkt:
		default:
		}

	case StreamTypeSync:
		select {
		case s.SyncCh <- pkt:
		default:
		}
	}
}

// WriteVideoTo reads video packets from the stream and writes them to w.
// Blocks until the context is cancelled or an error occurs.
func (s *Streamer) WriteVideoTo(ctx context.Context, w io.Writer) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case pkt, ok := <-s.VideoCh:
			if !ok {
				return nil
			}
			if _, err := w.Write(pkt.Data); err != nil {
				return fmt.Errorf("write video data: %w", err)
			}
		}
	}
}
