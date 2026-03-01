package direct

import (
	"context"
	"fmt"
	"sync"

	"github.com/facebookincubator/go-belt/tool/logger"
	"github.com/xaionaro-go/insta360ctl/pkg/ble"
	"github.com/xaionaro-go/insta360ctl/pkg/camera"
	"github.com/xaionaro-go/insta360ctl/pkg/protocol"
	"github.com/xaionaro-go/insta360ctl/pkg/protocol/messagecode"
	pb "github.com/xaionaro-go/insta360ctl/pkg/protocol/protobuf"
	"google.golang.org/protobuf/proto"
)

// Device represents an Insta360 camera found via BLE.
// After discovery, call Init() to connect and set up communication.
type Device struct {
	Adapter ble.Adapter
	ID      ble.DeviceID
	Cam     camera.Model
	BLName  string
	RSSI    int
	Address string // raw address string for connecting

	session *Session

	periph    ble.Peripheral
	charWrite ble.Characteristic
	services  []ble.Service
	allChars  []ble.Characteristic

	mu             sync.Mutex
	responseChs    map[uint8]chan []byte // keyed by sequence number (both formats)
	notificationCh chan []byte
	syncCh         chan []byte // receives sync responses
	go2SeqCounter  uint8      // sequence counter for Go2 format (1-254)

	// Cached data from unsolicited notifications (GO 3).
	cachedStorageInfo *camera.StorageInfo
}

// Model returns the camera model.
func (d *Device) Model() camera.Model { return d.Cam }

// Name returns the BLE advertised name.
func (d *Device) Name() string { return d.BLName }

func (d *Device) String() string {
	return fmt.Sprintf("%s [%s] (%s, RSSI: %d)", d.BLName, d.ID, d.Cam, d.RSSI)
}

// Init connects to the camera and initializes communication.
//
// Sequence: connect → discover services → set MTU → subscribe → sync → authorize.
func (d *Device) Init(ctx context.Context) error {
	logger.Infof(ctx, "connecting to %s...", d)

	periph, err := d.Adapter.Connect(ctx, d.Address)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	d.periph = periph
	logger.Infof(ctx, "connected to %s", d)

	// Discover services.
	logger.Debugf(ctx, "discovering services on %s", d)
	services, err := periph.DiscoverServices(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover services: %w", err)
	}
	d.services = services

	// Discover all characteristics and log them.
	for _, svc := range services {
		logger.Infof(ctx, "found service: uuid=%s", svc.UUID())
		chars, err := svc.DiscoverCharacteristics(ctx)
		if err != nil {
			logger.Warnf(ctx, "failed to discover characteristics for service %s: %v", svc.UUID(), err)
			continue
		}
		for _, c := range chars {
			logger.Infof(ctx, "  char: uuid=%s props=0x%02X", c.UUID(), c.Properties())
			d.allChars = append(d.allChars, c)
		}
	}

	// Set MTU for larger payloads.
	logger.Debugf(ctx, "setting MTU to 517")
	if err := periph.SetMTU(ctx, 517); err != nil {
		logger.Warnf(ctx, "failed to set MTU to 517: %v", err)
	}

	// Find write characteristic based on camera model.
	if d.Cam.DirectProtoFormat() == camera.ProtoFormatFFFrame {
		// GO 2/GO 3: use BE81 (primary BLE command characteristic).
		// Earlier code tried B001 first, but real-device testing with GO 3
		// shows that BE81 is the correct write characteristic — the camera
		// only responds to commands on BE81, not B001.
		d.charWrite = ble.FindCharByUUID16(d.allChars, protocol.UUIDDirectCharWrite)
		if d.charWrite == nil {
			d.charWrite = ble.FindCharByUUID16(d.allChars, protocol.UUIDAECharWrite)
		}
		if d.charWrite == nil {
			d.charWrite = ble.FindCharByUUID16(d.allChars, protocol.UUIDDirectSecCharWrite)
		}
		if d.charWrite == nil {
			return fmt.Errorf("write characteristic not found (tried BE81, AE01, and B001)")
		}
		logger.Infof(ctx, "using write characteristic: uuid=%s", d.charWrite.UUID())
	} else {
		// X3/ONE R: use BE81.
		d.charWrite = ble.FindCharByUUID16(d.allChars, protocol.UUIDDirectCharWrite)
		if d.charWrite == nil {
			return fmt.Errorf("BE81 write characteristic not found; %s (%s) may not support direct BLE control",
				d.BLName, d.Cam)
		}
	}

	// Subscribe to all notify characteristics.
	notifyUUIDs := []uint16{
		protocol.UUIDDirectCharNotify,    // BE82
		protocol.UUIDAECharNotify,        // AE02
		protocol.UUIDDirectSecCharNotify1, // B002
		protocol.UUIDDirectSecCharNotify2, // B003
		protocol.UUIDDirectSecCharNotify3, // B004
	}
	for _, uuid := range notifyUUIDs {
		c := ble.FindCharByUUID16(d.allChars, uuid)
		if c != nil {
			if err := c.EnableNotifications(d.makeNotifyHandler(uuid)); err != nil {
				logger.Warnf(ctx, "failed to subscribe to 0x%04X notifications: %v", uuid, err)
			} else {
				logger.Debugf(ctx, "subscribed to notify: uuid=0x%04X", uuid)
			}
		}
	}

	// For GO 2/GO 3: perform sync handshake and authorization.
	if d.Cam.DirectProtoFormat() == camera.ProtoFormatFFFrame {
		if err := d.syncHandshake(ctx); err != nil {
			logger.Warnf(ctx, "sync handshake failed (continuing anyway): %v", err)
		}

		if err := d.authorize(ctx); err != nil {
			logger.Warnf(ctx, "authorization failed (continuing anyway): %v", err)
		}
	}

	logger.Infof(ctx, "initialized device %s", d)
	return nil
}

func (d *Device) makeNotifyHandler(uuid uint16) func([]byte) {
	return func(b []byte) {
		ctx := context.Background()
		logger.Debugf(ctx, "notification received: uuid=0x%04X len=%d data=%X", uuid, len(b), b)

		if d.Cam.DirectProtoFormat() == camera.ProtoFormatFFFrame {
			d.handleGo2BleNotification(ctx, b)
		} else {
			d.handleHeader16Notification(ctx, b)
		}
	}
}

// syncHandshake handles the sync exchange with the camera.
//
// On GO 3, the camera initiates sync: it sends a sync packet (with data
// like AB BA AB BA...) immediately after the first write to any characteristic.
// We must respond with our own sync packet (7 zero bytes).
//
// The flow is:
//  1. We send a command (e.g. CheckAuth) to BE81
//  2. Camera replies with a sync packet on BE82
//  3. We respond with our sync packet on BE81
//  4. Camera processes the original command and sends the response
//
// This method sets up the sync channel so handleGo2BleNotification can
// auto-respond to camera-initiated sync.
func (d *Device) syncHandshake(ctx context.Context) error {
	logger.Infof(ctx, "setting up sync handshake (camera-initiated)...")

	d.syncCh = make(chan []byte, 1)

	// The sync response will be sent automatically by handleGo2BleNotification
	// when it receives a sync packet from the camera. We just wait for it to happen.
	// The camera sends sync after our first command write, so we don't need to
	// explicitly trigger it here — it will happen during authorize().

	return nil
}

// authorize sends a CHECK_AUTHORIZATION command to the camera using protobuf encoding.
func (d *Device) authorize(ctx context.Context) error {
	authID := d.ID.String()
	logger.Infof(ctx, "sending CheckAuthorization: id=%s", authID)

	msg := &pb.CheckAuthorization{
		AuthorizationId: authID,
		InitiatorType:   pb.CheckAuthorization_InitiatorType_INITIATOR_TYPE_APP,
	}
	payload, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal CheckAuthorization: %w", err)
	}
	logger.Debugf(ctx, "CheckAuthorization protobuf: %X", payload)

	resp, err := d.sendCommandGo2Ble(ctx, messagecode.CodeCheckAuthorization, payload)
	if err != nil {
		return fmt.Errorf("CheckAuthorization failed: %w", err)
	}

	respPayload, err := d.extractPayload(resp)
	if err != nil {
		logger.Warnf(ctx, "failed to extract auth response payload: %v (raw=%X)", err, resp)
		return nil
	}

	var authResp pb.CheckAuthorizationResp
	if err := proto.Unmarshal(respPayload, &authResp); err != nil {
		logger.Warnf(ctx, "failed to unmarshal CheckAuthorizationResp: %v (raw=%X)", err, respPayload)
		return nil
	}

	logger.Infof(ctx, "authorization response: status=%s findmy=%d",
		authResp.AuthorizationStatus, authResp.FindmyPairStatus)

	if authResp.AuthorizationStatus == pb.CheckAuthorizationResp_AuthorizationStatus_AUTHORIZATION_STATUS_NOT_AUTHORIZED {
		logger.Infof(ctx, "camera reports not authorized, sending RequestAuthorization...")
		return d.requestAuthorization(ctx)
	}

	return nil
}

// requestAuthorization sends a REQUEST_AUTHORIZATION command (triggers user confirmation on camera).
func (d *Device) requestAuthorization(ctx context.Context) error {
	msg := &pb.RequestAuthorization{
		OperationType: pb.AuthorizationOperationType_AUTHORIZATION_OPERATION_PAIR,
	}
	payload, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal RequestAuthorization: %w", err)
	}

	logger.Infof(ctx, "sending RequestAuthorization (user must confirm on camera)...")
	resp, err := d.sendCommandGo2Ble(ctx, messagecode.CodeRequestAuthorization, payload)
	if err != nil {
		return fmt.Errorf("RequestAuthorization failed: %w", err)
	}

	respPayload, err := d.extractPayload(resp)
	if err != nil {
		logger.Warnf(ctx, "failed to extract RequestAuthorization response: %v (raw=%X)", err, resp)
		return nil
	}

	var result pb.NotificationAuthorizationResult
	if err := proto.Unmarshal(respPayload, &result); err != nil {
		logger.Warnf(ctx, "failed to unmarshal authorization result: %v (raw=%X)", err, respPayload)
		return nil
	}

	if !result.Authorized {
		return fmt.Errorf("authorization denied by user")
	}

	logger.Infof(ctx, "authorization granted by user")
	return nil
}

func (d *Device) handleGo2BleNotification(ctx context.Context, b []byte) {
	innerData, _, subtypeByte, err := protocol.DecodeGo2BlePacket(b)
	if err != nil {
		logger.Debugf(ctx, "Go2BlePacket decode failed: %v (trying raw inner header)", err)
		d.handleGo2InnerData(ctx, b)
		return
	}

	logger.Debugf(ctx, "Go2BlePacket: subtype=0x%02X innerLen=%d inner=%X",
		subtypeByte, len(innerData), innerData)

	if subtypeByte == protocol.Go2BLESubtypeSync {
		logger.Infof(ctx, "received camera sync: %X — sending sync response", innerData)

		// Auto-respond with our sync packet (7 zero bytes).
		syncResp := protocol.EncodeGo2BleSyncPacket(make([]byte, 7))
		if err := d.charWrite.Write(syncResp, false); err != nil {
			logger.Warnf(ctx, "failed to send sync response: %v", err)
		} else {
			logger.Infof(ctx, "sync response sent: %X", syncResp)
		}

		if d.syncCh != nil {
			select {
			case d.syncCh <- innerData:
			default:
			}
		}
		return
	}

	d.handleGo2InnerData(ctx, innerData)
}

func (d *Device) handleGo2InnerData(ctx context.Context, innerData []byte) {
	hdr, err := protocol.DecodeGo2Header(innerData)
	if err != nil {
		logger.Debugf(ctx, "Go2 inner header decode failed: %v (forwarding as raw)", err)
		select {
		case d.notificationCh <- innerData:
		default:
		}
		return
	}

	logger.Debugf(ctx, "Go2 inner: cmd=0x%04X(%s) seq=%d payloadLen=%d",
		uint16(hdr.CommandCode), hdr.CommandCode, hdr.Sequence, hdr.PayloadLength)

	// Route by sequence number — responses echo the request's seq.
	d.mu.Lock()
	ch, ok := d.responseChs[hdr.Sequence]
	d.mu.Unlock()

	if ok {
		select {
		case ch <- innerData:
		default:
		}
	} else {
		// Unsolicited notification (seq=255 or no matching request).
		logger.Debugf(ctx, "unsolicited Go2 notification: cmd=0x%04X seq=%d",
			uint16(hdr.CommandCode), hdr.Sequence)

		// Cache certain notification types for later queries.
		payload := innerData[protocol.HeaderSize:]
		if int(hdr.PayloadLength) < len(payload) {
			payload = payload[:hdr.PayloadLength]
		}
		d.cacheNotificationData(ctx, hdr.CommandCode, payload)

		select {
		case d.notificationCh <- innerData:
		default:
		}
	}
}

func (d *Device) handleHeader16Notification(ctx context.Context, b []byte) {
	hdr, err := protocol.DecodeHeader(b)
	if err != nil {
		logger.Debugf(ctx, "header16 decode failed: %v (forwarding as raw)", err)
		select {
		case d.notificationCh <- b:
		default:
		}
		return
	}

	logger.Debugf(ctx, "header16: cmd=0x%02X seq=%d payloadLen=%d", hdr.CommandCode, hdr.Sequence, hdr.PayloadLength)

	d.mu.Lock()
	ch, ok := d.responseChs[hdr.Sequence]
	d.mu.Unlock()

	if ok {
		select {
		case ch <- b:
		default:
		}
	} else {
		logger.Debugf(ctx, "no response channel for seq=%d (unsolicited)", hdr.Sequence)
		select {
		case d.notificationCh <- b:
		default:
		}
	}
}

// SendCommand sends a command and waits for the response.
func (d *Device) SendCommand(ctx context.Context, cmd messagecode.Code, payload []byte) ([]byte, error) {
	if d.Cam.DirectProtoFormat() == camera.ProtoFormatFFFrame {
		return d.sendCommandGo2Ble(ctx, cmd, payload)
	}
	return d.sendCommandHeader16(ctx, cmd, payload)
}

// SendCommandNoResponse sends a command without waiting for a response.
func (d *Device) SendCommandNoResponse(ctx context.Context, cmd messagecode.Code, payload []byte) error {
	if d.Cam.DirectProtoFormat() == camera.ProtoFormatFFFrame {
		return d.sendNoResponseGo2Ble(ctx, cmd, payload)
	}
	return d.sendNoResponseHeader16(ctx, cmd, payload)
}

// --- Go2BlePacket protocol (GO 2, GO 3) ---

// nextGo2Seq returns the next sequence number (1-254).
func (d *Device) nextGo2Seq() uint8 {
	d.go2SeqCounter++
	if d.go2SeqCounter > 254 {
		d.go2SeqCounter = 1
	}
	return d.go2SeqCounter
}

func (d *Device) sendCommandGo2Ble(ctx context.Context, cmd messagecode.Code, payload []byte) ([]byte, error) {
	seq := d.nextGo2Seq()

	respCh := make(chan []byte, 1)
	d.mu.Lock()
	d.responseChs[seq] = respCh
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		delete(d.responseChs, seq)
		d.mu.Unlock()
	}()

	innerMsg := protocol.EncodeGo2Message(cmd, seq, payload)
	packet := protocol.EncodeGo2BleMessagePacket(innerMsg)
	logger.Debugf(ctx, "sending Go2BLE cmd=%s(0x%04X) seq=%d payload=%X packet=%X",
		cmd, uint16(cmd), seq, payload, packet)

	if err := d.charWrite.Write(packet, false); err != nil {
		return nil, fmt.Errorf("failed to write Go2BlePacket: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respCh:
		return resp, nil
	}
}

func (d *Device) sendNoResponseGo2Ble(ctx context.Context, cmd messagecode.Code, payload []byte) error {
	seq := d.nextGo2Seq()
	innerMsg := protocol.EncodeGo2Message(cmd, seq, payload)
	packet := protocol.EncodeGo2BleMessagePacket(innerMsg)
	logger.Debugf(ctx, "sending Go2BLE cmd=%s(0x%04X) seq=%d (no response) packet=%X", cmd, uint16(cmd), seq, packet)

	return d.charWrite.Write(packet, false)
}

// --- Header16 protocol (X3, ONE R, ONE RS) ---

func (d *Device) sendCommandHeader16(ctx context.Context, cmd messagecode.Code, payload []byte) ([]byte, error) {
	seq := d.session.NextSequence()
	msg := protocol.EncodeMessage(cmd, seq, payload)

	respCh := make(chan []byte, 1)
	d.mu.Lock()
	d.responseChs[seq] = respCh
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		delete(d.responseChs, seq)
		d.mu.Unlock()
	}()

	logger.Debugf(ctx, "sending header16 cmd=%s seq=%d payload=%d bytes raw=%X", cmd, seq, len(payload), msg)

	chunks := protocol.ChunkForBLE(msg, protocol.BLEMaxPacketSize)
	for i, chunk := range chunks {
		logger.Debugf(ctx, "  writing chunk %d/%d: %X", i+1, len(chunks), chunk)
		if err := d.charWrite.Write(chunk, false); err != nil {
			return nil, fmt.Errorf("failed to write chunk: %w", err)
		}
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respCh:
		return resp, nil
	}
}

func (d *Device) sendNoResponseHeader16(ctx context.Context, cmd messagecode.Code, payload []byte) error {
	seq := d.session.NextSequence()
	msg := protocol.EncodeMessage(cmd, seq, payload)

	chunks := protocol.ChunkForBLE(msg, protocol.BLEMaxPacketSize)
	for _, chunk := range chunks {
		if err := d.charWrite.Write(chunk, false); err != nil {
			return fmt.Errorf("failed to write chunk: %w", err)
		}
	}
	return nil
}

// Close disconnects from the camera.
func (d *Device) Close(ctx context.Context) error {
	if d.periph != nil {
		return d.periph.Disconnect(ctx)
	}
	return nil
}
