package direct

import (
	"context"
	"fmt"

	"github.com/facebookincubator/go-belt/tool/logger"
	"github.com/xaionaro-go/insta360ctl/pkg/protocol/messagecode"
	pb "github.com/xaionaro-go/insta360ctl/pkg/protocol/protobuf"
	"google.golang.org/protobuf/proto"
)

// WifiJoinResult holds the result of a BLE WiFi configuration command.
type WifiJoinResult struct {
	// Accepted is true if the camera acknowledged the command.
	Accepted bool
	// RawResponse is the raw response bytes (for debugging).
	RawResponse []byte
}

// SetWifiConfig sends WiFi network credentials to the camera over BLE
// using the official SET_WIFI_CONNECTION_INFO command (code 0x70 / 112).
//
// The protobuf payload is SetWifiConnectionInfo { WifiConnectionInfo { ssid, password } }.
//
// NOTE: The previous implementation used command 0x1A, which is actually
// SCAN_BT_PERIPHERAL and was a no-op on GO 3. The correct command is 0x70.
func (d *Device) SetWifiConfig(ctx context.Context, ssid, password string) (*WifiJoinResult, error) {
	msg := &pb.SetWifiConnectionInfo{
		WifiConnectionInfo: &pb.WifiConnectionInfo{
			Ssid:     ssid,
			Password: password,
		},
	}

	payload, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal SetWifiConnectionInfo: %w", err)
	}

	logger.Infof(ctx, "sending SetWifiConnectionInfo (0x70): ssid=%q payload=%X", ssid, payload)

	resp, err := d.SendCommand(ctx, messagecode.CodeSetWifiConnectionInfo, payload)
	if err != nil {
		return nil, fmt.Errorf("SetWifiConnectionInfo failed: %w", err)
	}

	logger.Debugf(ctx, "SetWifiConnectionInfo raw response: %X", resp)

	// Try to extract payload and check status.
	respPayload, err := d.extractPayload(resp)
	if err != nil {
		logger.Warnf(ctx, "SetWifiConnectionInfo response extraction failed: %v (raw=%X)", err, resp)
		return &WifiJoinResult{
			Accepted:    false,
			RawResponse: resp,
		}, fmt.Errorf("command not accepted: %w", err)
	}

	logger.Infof(ctx, "SetWifiConnectionInfo accepted, response payload: %X", respPayload)

	return &WifiJoinResult{
		Accepted:    true,
		RawResponse: resp,
	}, nil
}

// GetWifiConfig queries the camera's current WiFi configuration over BLE
// using the official GET_WIFI_CONNECTION_INFO command (code 0x71 / 113).
func (d *Device) GetWifiConfig(ctx context.Context) ([]byte, error) {
	logger.Infof(ctx, "sending GetWifiConnectionInfo (0x71)")

	resp, err := d.SendCommand(ctx, messagecode.CodeGetWifiConnectionInfo, nil)
	if err != nil {
		return nil, fmt.Errorf("GetWifiConnectionInfo failed: %w", err)
	}

	logger.Debugf(ctx, "GetWifiConnectionInfo raw response: %X", resp)

	respPayload, err := d.extractPayload(resp)
	if err != nil {
		return nil, fmt.Errorf("GetWifiConnectionInfo response: %w", err)
	}

	return respPayload, nil
}

// SetWifiMode sets the camera's WiFi operating mode (AP, STA, or P2P).
// Uses the official SET_WIFI_MODE command (code 0x93 / 147).
//
// Mode values: 0=AP (camera as hotspot), 1=STA (join existing network), 2=P2P.
func (d *Device) SetWifiMode(ctx context.Context, mode pb.WifiMode) error {
	msg := &pb.SetWifiMode{
		WifiMode: mode,
	}

	payload, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal SetWifiMode: %w", err)
	}

	logger.Infof(ctx, "sending SetWifiMode (0x93): mode=%s payload=%X", mode, payload)

	resp, err := d.SendCommand(ctx, messagecode.CodeSetWifiMode, payload)
	if err != nil {
		return fmt.Errorf("SetWifiMode failed: %w", err)
	}

	_, err = d.extractPayload(resp)
	if err != nil {
		return fmt.Errorf("SetWifiMode response: %w", err)
	}

	logger.Infof(ctx, "SetWifiMode accepted")
	return nil
}

// GetWifiMode queries the camera's current WiFi operating mode.
// Uses the official GET_WIFI_MODE command (code 0x96 / 150).
func (d *Device) GetWifiMode(ctx context.Context) (pb.WifiMode, error) {
	logger.Infof(ctx, "sending GetWifiMode (0x96)")

	resp, err := d.SendCommand(ctx, messagecode.CodeGetWifiMode, nil)
	if err != nil {
		return 0, fmt.Errorf("GetWifiMode failed: %w", err)
	}

	respPayload, err := d.extractPayload(resp)
	if err != nil {
		return 0, fmt.Errorf("GetWifiMode response: %w", err)
	}

	var result pb.GetWifiModeResp
	if err := proto.Unmarshal(respPayload, &result); err != nil {
		return 0, fmt.Errorf("unmarshal GetWifiModeResp: %w", err)
	}

	logger.Infof(ctx, "GetWifiMode: mode=%s", result.WifiMode)
	return result.WifiMode, nil
}

// GetWifiScanList asks the camera to scan for WiFi networks.
// Uses the official GET_WIFI_SCAN_LIST command (code 0x94 / 148).
func (d *Device) GetWifiScanList(ctx context.Context) error {
	msg := &pb.GetWifiScanInfo{
		Interval: 5,
		Count:    1,
	}

	payload, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal GetWifiScanInfo: %w", err)
	}

	logger.Infof(ctx, "sending GetWifiScanList (0x94)")

	resp, err := d.SendCommand(ctx, messagecode.CodeGetWifiScanList, payload)
	if err != nil {
		return fmt.Errorf("GetWifiScanList failed: %w", err)
	}

	_, err = d.extractPayload(resp)
	if err != nil {
		return fmt.Errorf("GetWifiScanList response: %w", err)
	}

	logger.Infof(ctx, "GetWifiScanList accepted — results will arrive via notification 0x2039")
	return nil
}

// ResetWifi resets the camera's WiFi configuration.
// Uses the official RESET_WIFI command (code 0x7D / 125).
func (d *Device) ResetWifi(ctx context.Context) error {
	logger.Infof(ctx, "sending ResetWifi (0x7D)")

	resp, err := d.SendCommand(ctx, messagecode.CodeResetWifi, nil)
	if err != nil {
		return fmt.Errorf("ResetWifi failed: %w", err)
	}

	_, err = d.extractPayload(resp)
	if err != nil {
		return fmt.Errorf("ResetWifi response: %w", err)
	}

	logger.Infof(ctx, "ResetWifi accepted")
	return nil
}
