package direct

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/facebookincubator/go-belt/tool/logger"
	"github.com/xaionaro-go/insta360ctl/pkg/camera"
	"github.com/xaionaro-go/insta360ctl/pkg/protocol"
	"github.com/xaionaro-go/insta360ctl/pkg/protocol/messagecode"
)

// extractPayload extracts the payload from a response based on the camera's protocol format.
// For GO 3: responses arrive as inner data (Go2BlePacket envelope already stripped by handler).
//
// The response uses a status code in the command field:
//   - 0x00C8 (200) = success
//   - 0x0190 (400) = unknown command
//   - 0x01F4 (500) = execution error
func (d *Device) extractPayload(resp []byte) ([]byte, error) {
	if d.Cam.DirectProtoFormat() == camera.ProtoFormatFFFrame {
		// GO 2/GO 3: inner data uses Go2 header format.
		hdr, payload, err := protocol.DecodeGo2Message(resp)
		if err != nil {
			return nil, fmt.Errorf("failed to decode Go2 message response: %w", err)
		}
		// Check response status code.
		switch hdr.CommandCode {
		case messagecode.CodeResponseOK:
			return payload, nil
		case messagecode.CodeResponseBadRequest:
			errMsg := "unknown command"
			if len(payload) > 2 {
				// field 2 (tag 0x12) = error message string
				errMsg = extractProtoString(payload)
			}
			return nil, fmt.Errorf("camera returned 400: %s", errMsg)
		case messagecode.CodeResponseError:
			errMsg := "execution error"
			if len(payload) > 2 {
				errMsg = extractProtoString(payload)
			}
			return nil, fmt.Errorf("camera returned 500: %s", errMsg)
		case messagecode.CodeResponseNotImpl:
			return nil, fmt.Errorf("camera returned 501: not implemented")
		default:
			// For non-status responses (e.g. unsolicited notifications),
			// just return the payload as-is.
			return payload, nil
		}
	}
	// X3/ONE R: Header16 format (uint16 payload length at bytes 0-1).
	_, payload, err := protocol.DecodeMessage(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode header16 response: %w", err)
	}
	return payload, nil
}

// extractProtoString tries to extract a string from a simple protobuf message.
// Looks for field 2 (tag 0x12) with a length-delimited string.
func extractProtoString(pb []byte) string {
	for i := 0; i < len(pb)-1; i++ {
		if pb[i] == 0x12 { // field 2, wire type 2 (length-delimited)
			strLen := int(pb[i+1])
			start := i + 2
			if start+strLen <= len(pb) {
				return string(pb[start : start+strLen])
			}
		}
	}
	return fmt.Sprintf("(raw: %X)", pb)
}

// GetBatteryInfo queries the camera's battery status.
func (d *Device) GetBatteryInfo(ctx context.Context) (*camera.BatteryInfo, error) {
	// GO 3 uses a different command code (0x19) than X3 (0x12).
	cmd := messagecode.CodeGetBatteryInfo
	if d.Cam.DirectProtoFormat() == camera.ProtoFormatFFFrame {
		cmd = messagecode.CodeGo3GetBattery
	}

	resp, err := d.SendCommand(ctx, cmd, nil)
	if err != nil {
		return nil, fmt.Errorf("get battery info failed: %w", err)
	}

	payload, err := d.extractPayload(resp)
	if err != nil {
		return nil, err
	}

	info := &camera.BatteryInfo{}

	if d.Cam.DirectProtoFormat() == camera.ProtoFormatFFFrame {
		// GO 3 response is protobuf: field 1 (varint) = battery %, field 2 (varint) = charging.
		// Decode simple varint fields.
		info.Level, info.Charging = parseGo3BatteryProtobuf(payload)
		logger.Debugf(ctx, "battery (GO3): %d%% charging=%v", info.Level, info.Charging)
	} else {
		// X3/ONE R: raw bytes.
		if len(payload) >= 1 {
			info.Level = payload[0]
		}
		if len(payload) >= 3 {
			info.Voltage = binary.LittleEndian.Uint16(payload[1:3])
		}
		logger.Debugf(ctx, "battery: %d%%, %dmV", info.Level, info.Voltage)
	}

	return info, nil
}

// parseGo3BatteryProtobuf decodes the GO 3 battery response protobuf.
// Format: field 1 (varint) = battery level %, field 2 (varint) = 1 if charging.
func parseGo3BatteryProtobuf(pb []byte) (level uint8, charging bool) {
	i := 0
	for i < len(pb) {
		if i+1 >= len(pb) {
			break
		}
		tag := pb[i]
		fieldNum := tag >> 3
		wireType := tag & 0x07
		i++

		if wireType == 0 { // varint
			val := uint64(0)
			shift := uint(0)
			for i < len(pb) {
				b := pb[i]
				i++
				val |= uint64(b&0x7F) << shift
				if b&0x80 == 0 {
					break
				}
				shift += 7
			}
			switch fieldNum {
			case 1:
				level = uint8(val)
			case 2:
				charging = val != 0
			}
		} else {
			// Skip other wire types.
			break
		}
	}
	return
}

// GetStorageInfo queries the camera's storage status.
func (d *Device) GetStorageInfo(ctx context.Context) (*camera.StorageInfo, error) {
	if d.Cam.DirectProtoFormat() == camera.ProtoFormatFFFrame {
		return d.getStorageInfoGo3(ctx)
	}
	return d.getStorageInfoHeader16(ctx)
}

// getStorageInfoGo3 queries storage info on GO 3 cameras.
//
// The GO 3 does not support CodeGetStorageInfo (0x10) — it returns 500.
// Instead, the camera pushes storage state via unsolicited 0x2010 notifications.
// We cache the last notification and return it here. If no notification has been
// received yet, we return a zero StorageInfo (the camera usually sends one
// shortly after connection).
func (d *Device) getStorageInfoGo3(ctx context.Context) (*camera.StorageInfo, error) {
	d.mu.Lock()
	cached := d.cachedStorageInfo
	d.mu.Unlock()

	if cached != nil {
		logger.Debugf(ctx, "storage (GO3 cached): total=%dMB free=%dMB files=%d",
			cached.TotalMB, cached.FreeMB, cached.FileCount)
		return cached, nil
	}

	// No cached data — return an error indicating storage info isn't available yet.
	return nil, fmt.Errorf("storage info not yet available; GO 3 provides storage via push notifications (0x2010)")
}

// getStorageInfoHeader16 queries storage on X3/ONE R cameras.
func (d *Device) getStorageInfoHeader16(ctx context.Context) (*camera.StorageInfo, error) {
	resp, err := d.SendCommand(ctx, messagecode.CodeGetStorageInfo, nil)
	if err != nil {
		return nil, fmt.Errorf("get storage info failed: %w", err)
	}

	payload, err := d.extractPayload(resp)
	if err != nil {
		return nil, err
	}

	info := &camera.StorageInfo{}
	if len(payload) >= 4 {
		info.TotalMB = binary.LittleEndian.Uint32(payload[0:4])
	}
	if len(payload) >= 8 {
		info.FreeMB = binary.LittleEndian.Uint32(payload[4:8])
	}
	if len(payload) >= 12 {
		info.FileCount = binary.LittleEndian.Uint32(payload[8:12])
	}
	if len(payload) >= 13 {
		info.IsFormatted = payload[12] != 0
	}

	logger.Debugf(ctx, "storage: total=%dMB free=%dMB files=%d", info.TotalMB, info.FreeMB, info.FileCount)
	return info, nil
}

// GetDeviceInfo queries the camera's device information (firmware, serial).
func (d *Device) GetDeviceInfo(ctx context.Context) (firmware string, serial string, err error) {
	resp, err := d.SendCommand(ctx, messagecode.CodeGetDeviceInfo, nil)
	if err != nil {
		return "", "", fmt.Errorf("get device info failed: %w", err)
	}

	payload, err := d.extractPayload(resp)
	if err != nil {
		return "", "", err
	}

	// Device info payload: length-prefixed strings.
	if len(payload) < 1 {
		return "", "", nil
	}

	offset := 0
	if offset < len(payload) {
		fwLen := int(payload[offset])
		offset++
		if offset+fwLen <= len(payload) {
			firmware = string(payload[offset : offset+fwLen])
			offset += fwLen
		}
	}
	if offset < len(payload) {
		snLen := int(payload[offset])
		offset++
		if offset+snLen <= len(payload) {
			serial = string(payload[offset : offset+snLen])
		}
	}

	logger.Debugf(ctx, "device info: firmware=%s serial=%s", firmware, serial)
	return firmware, serial, nil
}

// GetCameraState queries the overall camera state.
func (d *Device) GetCameraState(ctx context.Context) ([]byte, error) {
	resp, err := d.SendCommand(ctx, messagecode.CodeGetCameraState, nil)
	if err != nil {
		return nil, fmt.Errorf("get camera state failed: %w", err)
	}

	payload, err := d.extractPayload(resp)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

// cacheNotificationData processes unsolicited notification data from the camera
// and caches relevant state (storage, battery, etc.) for later queries.
func (d *Device) cacheNotificationData(ctx context.Context, cmd messagecode.Code, payload []byte) {
	switch cmd {
	case messagecode.CodeGo2NotifyStorageState:
		info := parseGo3StorageNotification(payload)
		if info != nil {
			d.mu.Lock()
			d.cachedStorageInfo = info
			d.mu.Unlock()
			logger.Debugf(ctx, "cached storage notification: total=%dMB free=%dMB files=%d",
				info.TotalMB, info.FreeMB, info.FileCount)
		}
	}
}

// parseGo3StorageNotification decodes the 0x2010 storage state notification protobuf.
//
// Observed format (from real GO 3 traffic):
//
//	field 1 (varint): capture state (0=idle, 1=post-recording, 4=recording)
//	field 4 (varint): free capacity (unit TBD — may be remaining seconds or MB)
//	field 5 (varint): file count
//	field 6 (varint): total capacity (unit TBD)
func parseGo3StorageNotification(pb []byte) *camera.StorageInfo {
	info := &camera.StorageInfo{}
	i := 0
	for i < len(pb) {
		if i >= len(pb) {
			break
		}
		tag := pb[i]
		fieldNum := tag >> 3
		wireType := tag & 0x07
		i++

		if wireType == 0 { // varint
			val := uint64(0)
			shift := uint(0)
			for i < len(pb) {
				b := pb[i]
				i++
				val |= uint64(b&0x7F) << shift
				if b&0x80 == 0 {
					break
				}
				shift += 7
			}
			switch fieldNum {
			case 4:
				info.FreeMB = uint32(val)
			case 5:
				info.FileCount = uint32(val)
			case 6:
				info.TotalMB = uint32(val)
			}
		} else if wireType == 2 { // length-delimited — skip
			if i >= len(pb) {
				break
			}
			length := int(pb[i])
			i++
			i += length
		} else {
			break
		}
	}
	return info
}
