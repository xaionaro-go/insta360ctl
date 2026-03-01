package wifi

import (
	"context"
	"fmt"
	"time"

	"github.com/facebookincubator/go-belt/tool/logger"
	pb "github.com/xaionaro-go/insta360ctl/pkg/protocol/protobuf"
	"google.golang.org/protobuf/proto"
)

// JoinResult holds the outcome of a WiFi join attempt.
type JoinResult struct {
	// Success is true if the camera joined the network.
	Success bool
	// ResultCode is the raw WifiConnectionResult enum value.
	ResultCode pb.WifiConnectionResult
	// SSID is the network the camera connected to.
	SSID string
	// IPAddr is the camera's IP on the joined network (if successful).
	IPAddr string
}

// JoinNetwork tells the camera to connect to the specified WiFi network.
// This must be called while connected to the camera's own WiFi AP.
//
// The camera will attempt to join the target network and send back a
// notification (code 8232) with the result. If successful, the camera's
// AP connection will be lost shortly after.
//
// The timeout controls how long to wait for the camera's response.
func JoinNetwork(ctx context.Context, conn *Conn, ssid, password string, timeout time.Duration) (*JoinResult, error) {
	msg := &pb.SetWifiConnectionInfo{
		WifiConnectionInfo: &pb.WifiConnectionInfo{
			Ssid:     ssid,
			Password: password,
		},
	}

	seq, err := conn.SendCommand(CmdSetWifiConnectionInfo, msg)
	if err != nil {
		return nil, fmt.Errorf("send SetWifiConnectionInfo: %w", err)
	}

	logger.Infof(ctx, "SetWifiConnectionInfo sent (seq=%d), waiting for result...", seq)

	// Wait for either:
	// 1. A direct response (200 OK) confirming the command was accepted
	// 2. A notification (8232) with the actual connection result
	//
	// The camera typically sends an OK response first, then the notification
	// after attempting to connect.

	deadline := time.Now().Add(timeout)
	if err := conn.SetReadDeadline(deadline); err != nil {
		return nil, fmt.Errorf("set read deadline: %w", err)
	}
	defer func() { _ = conn.SetReadDeadline(time.Time{}) }()

	gotOK := false

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if time.Now().After(deadline) {
			if gotOK {
				return nil, fmt.Errorf("command accepted but timed out waiting for WiFi connection result")
			}
			return nil, fmt.Errorf("timed out waiting for response")
		}

		payload, err := conn.ReadRaw()
		if err != nil {
			if gotOK {
				// Connection may have dropped because the camera switched WiFi.
				// This is expected — the camera joined the network but we lost
				// the AP connection before receiving the notification.
				logger.Infof(ctx, "connection lost after command accepted (camera may have switched WiFi mode)")
				return &JoinResult{
					Success:    true,
					ResultCode: pb.WifiConnectionResult_WIFI_SUCCESS,
					SSID:       ssid,
				}, nil
			}
			return nil, fmt.Errorf("read response: %w", err)
		}

		if len(payload) < 3 {
			continue
		}

		switch payload[0] {
		case pktTypeMessage:
			resp, parseErr := parseMessagePayload(payload)
			if parseErr != nil {
				logger.Warnf(ctx, "failed to parse message: %v", parseErr)
				continue
			}

			// Check if this is the WiFi connection result notification.
			if resp.ResponseCode == NotifyWifiConnectionResult {
				return parseWifiConnectionResult(resp.Body, ssid)
			}

			// Check if this is the command response.
			if resp.Sequence == seq {
				if resp.IsOK() {
					logger.Infof(ctx, "command accepted, waiting for WiFi connection result...")
					gotOK = true
					continue
				}
				return nil, fmt.Errorf("command rejected: response code %d", resp.ResponseCode)
			}

		case pktTypeKeepalive, pktTypeSync, pktTypeStream:
			// Ignore.
		}
	}
}

func parseWifiConnectionResult(body []byte, ssid string) (*JoinResult, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("empty WiFi connection result body")
	}

	var result pb.CameraWifiConnectionResult
	if err := proto.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal CameraWifiConnectionResult: %w", err)
	}

	jr := &JoinResult{
		ResultCode: result.WifiConnectionResult,
		SSID:       ssid,
	}

	if result.WifiConnectionInfo != nil {
		if result.WifiConnectionInfo.IpAddr != "" {
			jr.IPAddr = result.WifiConnectionInfo.IpAddr
		}
		if result.WifiConnectionInfo.Ssid != "" {
			jr.SSID = result.WifiConnectionInfo.Ssid
		}
	}

	switch result.WifiConnectionResult {
	case pb.WifiConnectionResult_WIFI_SUCCESS:
		jr.Success = true
	case pb.WifiConnectionResult_WIFI_TIMEOUT:
		return jr, fmt.Errorf("camera timed out connecting to %q", jr.SSID)
	case pb.WifiConnectionResult_WIFI_ERROR_CONNECT_FAILED:
		return jr, fmt.Errorf("camera failed to connect to %q", jr.SSID)
	default:
		return jr, fmt.Errorf("unknown WiFi connection result: %d", result.WifiConnectionResult)
	}

	return jr, nil
}

// GetWifiInfo queries the camera's current WiFi connection info.
func GetWifiInfo(ctx context.Context, conn *Conn, timeout time.Duration) (*pb.WifiConnectionInfo, error) {
	seq, err := conn.SendCommand(CmdGetWifiConnectionInfo, &pb.GetWifiConnectionInfo{})
	if err != nil {
		return nil, fmt.Errorf("send GetWifiConnectionInfo: %w", err)
	}

	deadline := time.Now().Add(timeout)
	if err := conn.SetReadDeadline(deadline); err != nil {
		return nil, fmt.Errorf("set read deadline: %w", err)
	}
	defer func() { _ = conn.SetReadDeadline(time.Time{}) }()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		payload, err := conn.ReadRaw()
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}

		if len(payload) < 3 {
			continue
		}

		if payload[0] != pktTypeMessage {
			continue
		}

		resp, parseErr := parseMessagePayload(payload)
		if parseErr != nil {
			continue
		}

		if resp.Sequence != seq {
			continue
		}

		if resp.IsError() {
			return nil, fmt.Errorf("GetWifiConnectionInfo failed: response code %d", resp.ResponseCode)
		}

		var result pb.GetWifiConnectionInfoResp
		if len(resp.Body) > 0 {
			if err := proto.Unmarshal(resp.Body, &result); err != nil {
				return nil, fmt.Errorf("unmarshal GetWifiConnectionInfoResp: %w", err)
			}
		}

		return result.WifiConnectionInfo, nil
	}
}
