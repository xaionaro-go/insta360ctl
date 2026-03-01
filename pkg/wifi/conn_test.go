package wifi

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startMockServer creates a TCP server that accepts one connection and
// returns the server-side conn and a cleanup function.
func startMockServer(t *testing.T) (net.Listener, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	return ln, func() { ln.Close() }
}

func writePacketTo(conn net.Conn, payload []byte) error {
	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, uint32(len(payload)+4))
	if _, err := conn.Write(header); err != nil {
		return err
	}
	_, err := conn.Write(payload)
	return err
}

func readPacketFrom(conn net.Conn) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}
	pktLen := binary.LittleEndian.Uint32(header)
	if pktLen < 4 {
		return nil, io.ErrUnexpectedEOF
	}
	payload := make([]byte, pktLen-4)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func TestPacketFraming(t *testing.T) {
	ln, cleanup := startMockServer(t)
	defer cleanup()

	// Server side: accept and echo packets.
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		sconn, err := ln.Accept()
		if err != nil {
			return
		}
		defer sconn.Close()

		payload, err := readPacketFrom(sconn)
		if err != nil {
			return
		}
		_ = writePacketTo(sconn, payload)
	}()

	// Client side: send and receive.
	conn, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	defer conn.Close()

	c := &Conn{
		conn: conn,
		buf:  make([]byte, 0, readBufSize),
	}

	testData := []byte("hello insta360")
	err = c.writePacket(testData)
	require.NoError(t, err)

	resp, err := c.readPacket()
	require.NoError(t, err)
	assert.Equal(t, testData, resp)

	<-serverDone
}

func TestSyncHandshake(t *testing.T) {
	ln, cleanup := startMockServer(t)
	defer cleanup()

	// Server: accept, read sync, echo it back.
	serverReady := make(chan struct{})
	go func() {
		defer close(serverReady)
		sconn, err := ln.Accept()
		if err != nil {
			return
		}
		defer sconn.Close()

		payload, err := readPacketFrom(sconn)
		if err != nil {
			return
		}

		// Verify it's a sync packet.
		expected := append([]byte{pktTypeSync, 0x00, 0x00}, []byte(syncMagic)...)
		if len(payload) != len(expected) {
			return
		}

		// Echo it back.
		_ = writePacketTo(sconn, payload)
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	defer conn.Close()

	c := &Conn{
		conn: conn,
		buf:  make([]byte, 0, readBufSize),
	}

	ctx := context.Background()
	err = c.syncHandshake(ctx)
	assert.NoError(t, err)

	<-serverReady
}

func TestSyncHandshakeBadResponse(t *testing.T) {
	ln, cleanup := startMockServer(t)
	defer cleanup()

	go func() {
		sconn, err := ln.Accept()
		if err != nil {
			return
		}
		defer sconn.Close()

		_, _ = readPacketFrom(sconn)
		// Send wrong response.
		_ = writePacketTo(sconn, []byte{pktTypeSync, 0x00, 0x00, 'b', 'a', 'd'})
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	defer conn.Close()

	c := &Conn{
		conn: conn,
		buf:  make([]byte, 0, readBufSize),
	}

	ctx := context.Background()
	err = c.syncHandshake(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "length mismatch")
}

func TestCommandBuildAndParse(t *testing.T) {
	seq := uint32(42)
	body := []byte{0x10, 0x01}

	pkt := buildCommandPacket(CmdStartLiveStream, seq, body)

	assert.Equal(t, byte(pktTypeMessage), pkt[0])
	assert.Equal(t, byte(0x00), pkt[1])
	assert.Equal(t, byte(0x00), pkt[2])

	// Command code.
	cmdCode := binary.LittleEndian.Uint16(pkt[3:5])
	assert.Equal(t, CmdStartLiveStream, cmdCode)

	// Constant.
	assert.Equal(t, byte(0x02), pkt[5])

	// Sequence.
	gotSeq := uint32(pkt[6]) | uint32(pkt[7])<<8 | uint32(pkt[8])<<16
	assert.Equal(t, seq, gotSeq)

	// Flags.
	assert.Equal(t, byte(0x80), pkt[9])

	// Body.
	assert.Equal(t, body, pkt[commandHeaderSize:])

	// Parse it back.
	resp, err := parseMessagePayload(pkt)
	require.NoError(t, err)
	assert.Equal(t, CmdStartLiveStream, resp.ResponseCode)
	assert.Equal(t, seq, resp.Sequence)
	assert.Equal(t, body, resp.Body)
}

func TestParseMessagePayloadTooShort(t *testing.T) {
	_, err := parseMessagePayload([]byte{0x04, 0x00, 0x00})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

func TestStreamPacketDispatch(t *testing.T) {
	s := &Streamer{
		VideoCh:    make(chan StreamPacket, 4),
		GyroCh:     make(chan StreamPacket, 4),
		SyncCh:     make(chan StreamPacket, 4),
		ResponseCh: make(chan *ParsedResponse, 4),
	}

	ctx := context.Background()

	// Build a video stream payload.
	payload := make([]byte, 20)
	payload[0] = pktTypeStream
	payload[1] = 0x00
	payload[2] = 0x00
	payload[3] = StreamTypeVideo
	binary.LittleEndian.PutUint64(payload[4:12], 12345678)
	copy(payload[12:], []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x11, 0x22})

	s.dispatchStream(ctx, payload)

	select {
	case pkt := <-s.VideoCh:
		assert.Equal(t, StreamTypeVideo, pkt.Type)
		assert.Equal(t, uint64(12345678), pkt.Timestamp)
		assert.Equal(t, []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x11, 0x22}, pkt.Data)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for video packet")
	}

	// Build a gyro stream payload.
	gyroPayload := make([]byte, 13)
	gyroPayload[0] = pktTypeStream
	gyroPayload[3] = StreamTypeGyro
	binary.LittleEndian.PutUint64(gyroPayload[4:12], 99999)
	gyroPayload[12] = 0x42

	s.dispatchStream(ctx, gyroPayload)

	select {
	case pkt := <-s.GyroCh:
		assert.Equal(t, StreamTypeGyro, pkt.Type)
		assert.Equal(t, uint64(99999), pkt.Timestamp)
		assert.Equal(t, []byte{0x42}, pkt.Data)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for gyro packet")
	}

	// Build a sync stream payload.
	syncPayload := make([]byte, 12)
	syncPayload[0] = pktTypeStream
	syncPayload[3] = StreamTypeSync
	binary.LittleEndian.PutUint64(syncPayload[4:12], 55555)

	s.dispatchStream(ctx, syncPayload)

	select {
	case pkt := <-s.SyncCh:
		assert.Equal(t, StreamTypeSync, pkt.Type)
		assert.Equal(t, uint64(55555), pkt.Timestamp)
		assert.Nil(t, pkt.Data)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for sync packet")
	}
}

func TestDispatchStreamTooShort(t *testing.T) {
	s := &Streamer{
		VideoCh:    make(chan StreamPacket, 4),
		GyroCh:     make(chan StreamPacket, 4),
		SyncCh:     make(chan StreamPacket, 4),
		ResponseCh: make(chan *ParsedResponse, 4),
	}

	// Payload shorter than streamHeaderSize should be silently dropped.
	s.dispatchStream(context.Background(), []byte{0x01, 0x00, 0x00})

	select {
	case <-s.VideoCh:
		t.Fatal("should not have dispatched anything")
	case <-time.After(50 * time.Millisecond):
		// OK, nothing dispatched.
	}
}

func TestKeepAliveLoop(t *testing.T) {
	ln, cleanup := startMockServer(t)
	defer cleanup()

	received := make(chan []byte, 10)
	go func() {
		sconn, err := ln.Accept()
		if err != nil {
			return
		}
		defer sconn.Close()
		for {
			payload, err := readPacketFrom(sconn)
			if err != nil {
				return
			}
			received <- payload
		}
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	defer conn.Close()

	c := &Conn{
		conn: conn,
		buf:  make([]byte, 0, readBufSize),
	}

	ctx, cancel := context.WithCancel(context.Background())
	go c.keepAliveLoop(ctx)

	// Wait for at least one keepalive.
	select {
	case pkt := <-received:
		assert.Equal(t, []byte{pktTypeKeepalive, 0x00, 0x00}, pkt)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for keepalive")
	}

	cancel()
}

func TestWriteVideoTo(t *testing.T) {
	s := &Streamer{
		VideoCh: make(chan StreamPacket, 4),
	}

	// Push some video packets.
	s.VideoCh <- StreamPacket{Type: StreamTypeVideo, Data: []byte("frame1")}
	s.VideoCh <- StreamPacket{Type: StreamTypeVideo, Data: []byte("frame2")}

	ctx, cancel := context.WithCancel(context.Background())

	// Use a pipe to capture output.
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- s.WriteVideoTo(ctx, pw)
		pw.Close()
	}()

	buf := make([]byte, 12)
	n, err := io.ReadFull(pr, buf)
	require.NoError(t, err)
	assert.Equal(t, 12, n)
	assert.Equal(t, "frame1frame2", string(buf))

	cancel()
	<-done
}
