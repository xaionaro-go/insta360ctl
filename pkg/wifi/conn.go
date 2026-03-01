// Package wifi implements the Insta360 proprietary TCP protocol used for
// camera control and video preview streaming over WiFi.
//
// The camera runs a WiFi AP (IP 192.168.42.1) and serves a TCP protocol
// on port 6666 that multiplexes commands, responses, keep-alives, and
// stream data (video/gyro/sync) on a single connection.
package wifi

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/facebookincubator/go-belt/tool/logger"
)

const (
	DefaultAddr = "192.168.42.1"
	DefaultPort = 6666

	syncMagic          = "syNceNdinS"
	keepaliveInterval  = 2 * time.Second
	connectedTimeout   = 10 * time.Second
	maxPacketSize      = 1 << 20 // 1 MiB safety limit
	readBufSize        = 64 * 1024
)

// Packet type identifiers (first 3 bytes of payload).
const (
	pktTypeStream    byte = 0x01
	pktTypeMessage   byte = 0x04
	pktTypeKeepalive byte = 0x05
	pktTypeSync      byte = 0x06
)

// Stream type identifiers (byte 3 of stream payload).
const (
	StreamTypeVideo byte = 0x20
	StreamTypeGyro  byte = 0x30
	StreamTypeSync  byte = 0x40
)

// Conn is a connection to an Insta360 camera's WiFi TCP protocol.
type Conn struct {
	conn net.Conn

	mu       sync.Mutex
	lastSend time.Time

	readMu sync.Mutex
	buf    []byte

	cancelKeepAlive context.CancelFunc
}

// Dial connects to the camera's TCP protocol and performs the sync handshake.
func Dial(ctx context.Context, addr string, port int) (*Conn, error) {
	if addr == "" {
		addr = DefaultAddr
	}
	if port == 0 {
		port = DefaultPort
	}

	target := fmt.Sprintf("%s:%d", addr, port)
	logger.Infof(ctx, "connecting to %s...", target)

	dialer := net.Dialer{Timeout: 10 * time.Second}
	tcpConn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return nil, fmt.Errorf("TCP connect to %s: %w", target, err)
	}

	c := &Conn{
		conn: tcpConn,
		buf:  make([]byte, 0, readBufSize),
	}

	if err := c.syncHandshake(ctx); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("sync handshake: %w", err)
	}

	logger.Infof(ctx, "sync handshake complete")

	// Start keep-alive goroutine.
	kaCtx, kaCancel := context.WithCancel(ctx)
	c.cancelKeepAlive = kaCancel
	go c.keepAliveLoop(kaCtx)

	return c, nil
}

// Close gracefully shuts down the connection.
func (c *Conn) Close() error {
	if c.cancelKeepAlive != nil {
		c.cancelKeepAlive()
	}
	return c.conn.Close()
}

// writePacket sends a payload with the 4-byte length prefix.
// The length includes the 4 prefix bytes themselves.
func (c *Conn) writePacket(payload []byte) error {
	totalLen := uint32(len(payload) + 4)
	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, totalLen)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastSend = time.Now()

	if _, err := c.conn.Write(header); err != nil {
		return err
	}
	_, err := c.conn.Write(payload)
	return err
}

// readPacket reads a single length-prefixed packet and returns its payload.
func (c *Conn) readPacket() ([]byte, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	// Ensure we have at least the 4-byte length header.
	if err := c.fillBuf(4); err != nil {
		return nil, err
	}

	pktLen := binary.LittleEndian.Uint32(c.buf[:4])
	if pktLen < 4 {
		return nil, fmt.Errorf("invalid packet length: %d", pktLen)
	}
	if pktLen > maxPacketSize {
		return nil, fmt.Errorf("packet too large: %d bytes", pktLen)
	}

	// Read the full packet (length includes the 4-byte header).
	if err := c.fillBuf(int(pktLen)); err != nil {
		return nil, err
	}

	payload := make([]byte, pktLen-4)
	copy(payload, c.buf[4:pktLen])
	c.buf = c.buf[pktLen:]

	return payload, nil
}

// fillBuf ensures c.buf has at least n bytes.
func (c *Conn) fillBuf(n int) error {
	for len(c.buf) < n {
		tmp := make([]byte, readBufSize)
		nr, err := c.conn.Read(tmp)
		if nr > 0 {
			c.buf = append(c.buf, tmp[:nr]...)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// syncHandshake performs the initial SYNC exchange with the camera.
func (c *Conn) syncHandshake(ctx context.Context) error {
	syncPayload := append([]byte{pktTypeSync, 0x00, 0x00}, []byte(syncMagic)...)

	if err := c.writePacket(syncPayload); err != nil {
		return fmt.Errorf("send sync: %w", err)
	}

	logger.Debugf(ctx, "sync packet sent, waiting for echo...")

	// Read response with timeout.
	if err := c.conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("set read deadline: %w", err)
	}
	defer func() { _ = c.conn.SetReadDeadline(time.Time{}) }()

	resp, err := c.readPacket()
	if err != nil {
		return fmt.Errorf("read sync response: %w", err)
	}

	// Verify it's a sync echo.
	if len(resp) < 3 || resp[0] != pktTypeSync {
		return fmt.Errorf("unexpected response type: %X", resp)
	}

	expectedSync := append([]byte{pktTypeSync, 0x00, 0x00}, []byte(syncMagic)...)
	if len(resp) != len(expectedSync) {
		return fmt.Errorf("sync response length mismatch: got %d, want %d", len(resp), len(expectedSync))
	}
	for i := range expectedSync {
		if resp[i] != expectedSync[i] {
			return fmt.Errorf("sync response mismatch at byte %d: got 0x%02X, want 0x%02X", i, resp[i], expectedSync[i])
		}
	}

	return nil
}

// keepAliveLoop sends periodic keep-alive packets.
func (c *Conn) keepAliveLoop(ctx context.Context) {
	ticker := time.NewTicker(keepaliveInterval)
	defer ticker.Stop()

	keepalivePayload := []byte{pktTypeKeepalive, 0x00, 0x00}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.Lock()
			elapsed := time.Since(c.lastSend)
			c.mu.Unlock()

			if elapsed >= keepaliveInterval {
				if err := c.writePacket(keepalivePayload); err != nil {
					logger.Warnf(ctx, "keep-alive send failed: %v", err)
					return
				}
			}
		}
	}
}

// SendRaw writes a raw payload to the connection (with length prefix).
func (c *Conn) SendRaw(payload []byte) error {
	return c.writePacket(payload)
}

// ReadRaw reads a single raw payload from the connection.
func (c *Conn) ReadRaw() ([]byte, error) {
	return c.readPacket()
}

// RemoteAddr returns the remote address of the connection.
func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// SetReadDeadline sets the read deadline on the underlying connection.
func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

var _ io.Closer = (*Conn)(nil)
