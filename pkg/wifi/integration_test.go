package wifi

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCamera simulates an Insta360 camera's WiFi TCP server.
type mockCamera struct {
	ln   net.Listener
	conn net.Conn
}

func newMockCamera(t *testing.T) *mockCamera {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	return &mockCamera{ln: ln}
}

func (m *mockCamera) accept(t *testing.T) {
	t.Helper()
	var err error
	m.conn, err = m.ln.Accept()
	require.NoError(t, err)
}

func (m *mockCamera) read(t *testing.T) []byte {
	t.Helper()
	header := make([]byte, 4)
	_, err := m.conn.Read(header)
	require.NoError(t, err)
	pktLen := binary.LittleEndian.Uint32(header)
	require.True(t, pktLen >= 4)
	payload := make([]byte, pktLen-4)
	if len(payload) > 0 {
		n := 0
		for n < len(payload) {
			nr, err := m.conn.Read(payload[n:])
			require.NoError(t, err)
			n += nr
		}
	}
	return payload
}

func (m *mockCamera) write(t *testing.T, payload []byte) {
	t.Helper()
	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, uint32(len(payload)+4))
	_, err := m.conn.Write(header)
	require.NoError(t, err)
	_, err = m.conn.Write(payload)
	require.NoError(t, err)
}

func (m *mockCamera) close() {
	if m.conn != nil {
		m.conn.Close()
	}
	m.ln.Close()
}

func TestDialAndSync(t *testing.T) {
	cam := newMockCamera(t)
	defer cam.close()

	addr := cam.ln.Addr().(*net.TCPAddr)

	// Server goroutine: handle sync.
	go func() {
		cam.accept(t)
		// Read sync.
		payload := cam.read(t)
		// Echo it back.
		cam.write(t, payload)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := Dial(ctx, addr.IP.String(), addr.Port)
	require.NoError(t, err)
	defer c.Close()

	assert.NotNil(t, c.RemoteAddr())
}

func TestFullStreamSession(t *testing.T) {
	cam := newMockCamera(t)
	defer cam.close()

	addr := cam.ln.Addr().(*net.TCPAddr)

	videoData := []byte{0x00, 0x00, 0x00, 0x01, 0x65, 0xAA, 0xBB, 0xCC} // fake H.264 NAL

	go func() {
		cam.accept(t)

		// 1. Handle sync handshake.
		syncPayload := cam.read(t)
		cam.write(t, syncPayload)

		// 2. Read keepalive (may come before START_LIVE_STREAM).
		for {
			payload := cam.read(t)
			if len(payload) >= 3 && payload[0] == pktTypeKeepalive {
				continue // skip keepalives
			}
			// 3. Should be START_LIVE_STREAM command.
			if len(payload) >= commandHeaderSize && payload[0] == pktTypeMessage {
				cmdCode := binary.LittleEndian.Uint16(payload[3:5])
				if cmdCode == CmdStartLiveStream {
					// Send OK response.
					seq := [3]byte{payload[6], payload[7], payload[8]}
					resp := buildOKResponse(seq)
					cam.write(t, resp)

					// 4. Send some video stream data.
					for i := 0; i < 3; i++ {
						streamPkt := buildStreamPacket(StreamTypeVideo, uint64(i*33333), videoData)
						cam.write(t, streamPkt)
					}
					return
				}
			}
			break
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, err := Dial(ctx, addr.IP.String(), addr.Port)
	require.NoError(t, err)
	defer c.Close()

	streamer := NewStreamer(c)
	opts := DefaultStreamOptions()
	err = streamer.Start(ctx, opts)
	require.NoError(t, err)

	// Read 3 video packets.
	var buf bytes.Buffer
	for i := 0; i < 3; i++ {
		select {
		case pkt := <-streamer.VideoCh:
			assert.Equal(t, StreamTypeVideo, pkt.Type)
			buf.Write(pkt.Data)
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for video packet %d", i)
		}
	}

	// Verify total video data.
	assert.Equal(t, len(videoData)*3, buf.Len())
}

func TestResponseHelpers(t *testing.T) {
	ok := &ParsedResponse{ResponseCode: RespOK}
	assert.True(t, ok.IsOK())
	assert.False(t, ok.IsError())
	assert.False(t, ok.IsNotification())

	errResp := &ParsedResponse{ResponseCode: RespError}
	assert.False(t, errResp.IsOK())
	assert.True(t, errResp.IsError())
	assert.False(t, errResp.IsNotification())

	notif := &ParsedResponse{ResponseCode: NotifyBatteryLow}
	assert.False(t, notif.IsOK())
	assert.False(t, notif.IsError())
	assert.True(t, notif.IsNotification())
}

func TestSendCommand(t *testing.T) {
	cam := newMockCamera(t)
	defer cam.close()

	addr := cam.ln.Addr().(*net.TCPAddr)

	go func() {
		cam.accept(t)
		// Read sync, echo back.
		syncPayload := cam.read(t)
		cam.write(t, syncPayload)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := Dial(ctx, addr.IP.String(), addr.Port)
	require.NoError(t, err)
	defer c.Close()

	seq, err := c.SendCommand(CmdGetOptions, nil)
	require.NoError(t, err)
	assert.True(t, seq > 0)
}

// buildOKResponse creates a MESSAGE response with code 200.
func buildOKResponse(seq [3]byte) []byte {
	pkt := make([]byte, commandHeaderSize)
	pkt[0] = pktTypeMessage
	pkt[1] = 0x00
	pkt[2] = 0x00
	binary.LittleEndian.PutUint16(pkt[3:5], RespOK)
	pkt[5] = 0x02
	pkt[6] = seq[0]
	pkt[7] = seq[1]
	pkt[8] = seq[2]
	pkt[9] = 0x80
	return pkt
}

// buildStreamPacket creates a stream data packet.
func buildStreamPacket(streamType byte, timestamp uint64, data []byte) []byte {
	pkt := make([]byte, streamHeaderSize+len(data))
	pkt[0] = pktTypeStream
	pkt[1] = 0x00
	pkt[2] = 0x00
	pkt[3] = streamType
	binary.LittleEndian.PutUint64(pkt[4:12], timestamp)
	copy(pkt[streamHeaderSize:], data)
	return pkt
}
