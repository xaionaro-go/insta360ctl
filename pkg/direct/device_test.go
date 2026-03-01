package direct

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xaionaro-go/insta360ctl/pkg/ble"
	"github.com/xaionaro-go/insta360ctl/pkg/camera"
	"github.com/xaionaro-go/insta360ctl/pkg/protocol"
	"github.com/xaionaro-go/insta360ctl/pkg/protocol/messagecode"
)

// mockCharacteristic implements ble.Characteristic for testing.
type mockCharacteristic struct {
	uuid       string
	props      uint8
	mu         sync.Mutex
	written    [][]byte
	notifyCB   func([]byte)
	writeError error
}

func (c *mockCharacteristic) UUID() string { return c.uuid }
func (c *mockCharacteristic) Properties() ble.CharProps {
	return ble.CharProps(c.props)
}
func (c *mockCharacteristic) Write(data []byte, _ bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.writeError != nil {
		return c.writeError
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	c.written = append(c.written, cp)
	return nil
}
func (c *mockCharacteristic) Read() ([]byte, error) { return nil, nil }
func (c *mockCharacteristic) EnableNotifications(cb func([]byte)) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notifyCB = cb
	return nil
}
func (c *mockCharacteristic) lastWritten() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.written) == 0 {
		return nil
	}
	return c.written[len(c.written)-1]
}
// newTestDevice creates a Device with mock characteristics for testing.
func newTestDevice(model camera.Model) (*Device, *mockCharacteristic) {
	writeCh := &mockCharacteristic{
		uuid:  "0000be81-0000-1000-8000-00805f9b34fb",
		props: 0x04, // Write
	}

	return &Device{
		Cam:            model,
		session:        NewSession(),
		charWrite:      writeCh,
		responseChs:    make(map[uint8]chan []byte),
		notificationCh: make(chan []byte, 10),
	}, writeCh
}

func TestSendCommandGo2BleRoundtrip(t *testing.T) {
	dev, writeCh := newTestDevice(camera.ModelGo3)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Simulate a camera that responds with 200 OK to any command.
	go func() {
		// Wait for the device to write a command.
		time.Sleep(50 * time.Millisecond)

		lastWrite := writeCh.lastWritten()
		require.NotNil(t, lastWrite)

		// Decode the written Go2BLE packet to get the sequence number.
		innerData, _, _, err := protocol.DecodeGo2BlePacket(lastWrite)
		require.NoError(t, err)

		reqHdr, err := protocol.DecodeGo2Header(innerData)
		require.NoError(t, err)

		// Build a 200 OK response with the same sequence number.
		responsePayload := []byte{0x08, 0x64, 0x10, 0x01} // battery: 100%, charging
		responseInner := protocol.EncodeGo2Message(messagecode.CodeResponseOK, reqHdr.Sequence, responsePayload)

		// Send the response through the notification handler.
		dev.handleGo2InnerData(ctx, responseInner)
	}()

	resp, err := dev.sendCommandGo2Ble(ctx, messagecode.CodeGo3GetBattery, nil)
	require.NoError(t, err)

	// Verify we got the response inner data back.
	hdr, payload, err := protocol.DecodeGo2Message(resp)
	require.NoError(t, err)
	assert.Equal(t, messagecode.CodeResponseOK, hdr.CommandCode)
	assert.Equal(t, []byte{0x08, 0x64, 0x10, 0x01}, payload)
}

func TestSendCommandGo2BleTimeout(t *testing.T) {
	dev, _ := newTestDevice(camera.ModelGo3)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Send command without any response — should timeout.
	_, err := dev.sendCommandGo2Ble(ctx, messagecode.CodeGetOptions, nil)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestSendCommandHeader16Roundtrip(t *testing.T) {
	dev, writeCh := newTestDevice(camera.ModelX3)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		time.Sleep(50 * time.Millisecond)

		lastWrite := writeCh.lastWritten()
		require.NotNil(t, lastWrite)

		// Decode the written Header16 message.
		reqHdr, err := protocol.DecodeHeader(lastWrite)
		require.NoError(t, err)

		// Build response with same sequence number.
		responsePayload := []byte{0x50, 0x10, 0x27} // level=80, voltage=10000
		responseMsg := protocol.EncodeMessage(messagecode.CodeGetBatteryInfo, reqHdr.Sequence, responsePayload)

		// Send through notification handler.
		dev.handleHeader16Notification(ctx, responseMsg)
	}()

	resp, err := dev.sendCommandHeader16(ctx, messagecode.CodeGetBatteryInfo, nil)
	require.NoError(t, err)

	hdr, payload, err := protocol.DecodeMessage(resp)
	require.NoError(t, err)
	assert.Equal(t, messagecode.CodeGetBatteryInfo, hdr.CommandCode)
	assert.Equal(t, []byte{0x50, 0x10, 0x27}, payload)
}

func TestHandleGo2BleNotificationSync(t *testing.T) {
	dev, writeCh := newTestDevice(camera.ModelGo3)
	dev.syncCh = make(chan []byte, 1)
	ctx := context.Background()

	// Simulate camera sending a sync packet.
	syncData := []byte{0xAB, 0xBA, 0xAB, 0xBA, 0xAB, 0xBA, 0xAB, 0xBA, 0xAB, 0xBA}
	syncPacket := protocol.EncodeGo2BleSyncPacket(syncData)

	// Simulate the BLE notification arriving.
	dev.handleGo2BleNotification(ctx, syncPacket)

	// Verify sync data was forwarded to syncCh.
	select {
	case got := <-dev.syncCh:
		assert.Equal(t, syncData, got)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for sync data")
	}

	// Verify we responded with a sync packet (7 zero bytes).
	lastWrite := writeCh.lastWritten()
	require.NotNil(t, lastWrite)
	// Decode the sync response.
	innerData, _, subtypeByte, err := protocol.DecodeGo2BlePacket(lastWrite)
	require.NoError(t, err)
	assert.Equal(t, byte(protocol.Go2BLESubtypeSync), subtypeByte)
	assert.Equal(t, make([]byte, 7), innerData)
}

func TestHandleGo2InnerDataUnsolicitedNotification(t *testing.T) {
	dev, _ := newTestDevice(camera.ModelGo3)
	ctx := context.Background()

	// Build an unsolicited notification (seq=255) for periodic status.
	notifPayload := []byte{0x01, 0x02, 0x03}
	notifInner := protocol.EncodeGo2Message(messagecode.CodeGo2NotifyPeriodicStatus, 255, notifPayload)

	dev.handleGo2InnerData(ctx, notifInner)

	// Should arrive on the notification channel since no response channel for seq=255.
	select {
	case got := <-dev.notificationCh:
		assert.Equal(t, notifInner, got)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for unsolicited notification")
	}
}

func TestHandleGo2InnerDataRoutedBySequence(t *testing.T) {
	dev, _ := newTestDevice(camera.ModelGo3)
	ctx := context.Background()

	// Register a response channel for seq=42.
	respCh := make(chan []byte, 1)
	dev.mu.Lock()
	dev.responseChs[42] = respCh
	dev.mu.Unlock()

	// Send a response with seq=42.
	responseInner := protocol.EncodeGo2Message(messagecode.CodeResponseOK, 42, []byte{0xAA, 0xBB})
	dev.handleGo2InnerData(ctx, responseInner)

	// Should arrive on the registered channel.
	select {
	case got := <-respCh:
		assert.Equal(t, responseInner, got)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for routed response")
	}
}

func TestHandleHeader16NotificationRoutedBySequence(t *testing.T) {
	dev, _ := newTestDevice(camera.ModelX3)
	ctx := context.Background()

	respCh := make(chan []byte, 1)
	dev.mu.Lock()
	dev.responseChs[10] = respCh
	dev.mu.Unlock()

	responseMsg := protocol.EncodeMessage(messagecode.CodeGetBatteryInfo, 10, []byte{0x50})
	dev.handleHeader16Notification(ctx, responseMsg)

	select {
	case got := <-respCh:
		assert.Equal(t, responseMsg, got)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for routed response")
	}
}

func TestHandleHeader16NotificationUnsolicited(t *testing.T) {
	dev, _ := newTestDevice(camera.ModelX3)
	ctx := context.Background()

	// No response channel registered for seq=99.
	msg := protocol.EncodeMessage(messagecode.CodeNotifyBatteryUpdate, 99, []byte{0x60})
	dev.handleHeader16Notification(ctx, msg)

	select {
	case got := <-dev.notificationCh:
		assert.Equal(t, msg, got)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for unsolicited notification")
	}
}

func TestDeviceString(t *testing.T) {
	dev := &Device{
		BLName: "GO 3 74BC3Q",
		ID:     []byte{0xE5, 0xF2, 0x1D, 0x13, 0xBE, 0x34},
		Cam:    camera.ModelGo3,
		RSSI:   -40,
	}
	s := dev.String()
	assert.Contains(t, s, "GO 3 74BC3Q")
	assert.Contains(t, s, "GO 3")
	assert.Contains(t, s, "-40")
}

func TestDeviceModelAndName(t *testing.T) {
	dev := &Device{
		BLName: "X3 ABCDEF",
		Cam:    camera.ModelX3,
	}
	assert.Equal(t, camera.ModelX3, dev.Model())
	assert.Equal(t, "X3 ABCDEF", dev.Name())
}

func TestSendNoResponseGo2Ble(t *testing.T) {
	dev, writeCh := newTestDevice(camera.ModelGo3)
	ctx := context.Background()

	err := dev.sendNoResponseGo2Ble(ctx, messagecode.CodePowerOff, nil)
	require.NoError(t, err)

	// Verify a packet was written.
	lastWrite := writeCh.lastWritten()
	require.NotNil(t, lastWrite)

	// Decode and verify it's a valid Go2BLE packet with PowerOff command.
	innerData, _, _, err := protocol.DecodeGo2BlePacket(lastWrite)
	require.NoError(t, err)

	hdr, err := protocol.DecodeGo2Header(innerData)
	require.NoError(t, err)
	assert.Equal(t, messagecode.CodePowerOff, hdr.CommandCode)
}

func TestSendNoResponseHeader16(t *testing.T) {
	dev, writeCh := newTestDevice(camera.ModelX3)
	ctx := context.Background()

	err := dev.sendNoResponseHeader16(ctx, messagecode.CodePowerOff, nil)
	require.NoError(t, err)

	lastWrite := writeCh.lastWritten()
	require.NotNil(t, lastWrite)

	hdr, err := protocol.DecodeHeader(lastWrite)
	require.NoError(t, err)
	assert.Equal(t, messagecode.CodePowerOff, hdr.CommandCode)
}
