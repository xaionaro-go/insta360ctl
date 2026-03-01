package wifi

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pb "github.com/xaionaro-go/insta360ctl/pkg/protocol/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestJoinNetworkSuccess(t *testing.T) {
	cam := newMockCamera(t)
	defer cam.close()

	addr := cam.ln.Addr().(*net.TCPAddr)

	go func() {
		cam.accept(t)

		// 1. Handle sync.
		syncPayload := cam.read(t)
		cam.write(t, syncPayload)

		// 2. Skip keepalives, find the SetWifiConnectionInfo command.
		for {
			payload := cam.read(t)
			if len(payload) >= 3 && payload[0] == pktTypeKeepalive {
				continue
			}

			if len(payload) >= commandHeaderSize && payload[0] == pktTypeMessage {
				cmdCode := binary.LittleEndian.Uint16(payload[3:5])
				if cmdCode == CmdSetWifiConnectionInfo {
					seq := [3]byte{payload[6], payload[7], payload[8]}

					// Send OK response first.
					resp := buildOKResponse(seq)
					cam.write(t, resp)

					// Then send WiFi connection result notification.
					result := &pb.CameraWifiConnectionResult{
						WifiConnectionResult: pb.WifiConnectionResult_WIFI_SUCCESS,
						WifiConnectionInfo: &pb.WifiConnectionInfo{
							Ssid:   "TestNet",
							IpAddr: "192.168.1.42",
						},
					}
					body, err := proto.Marshal(result)
					require.NoError(t, err)

					notifPkt := buildNotificationPacket(NotifyWifiConnectionResult, body)
					cam.write(t, notifPkt)
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

	result, err := JoinNetwork(ctx, c, "TestNet", "password123", 10*time.Second)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "TestNet", result.SSID)
	assert.Equal(t, "192.168.1.42", result.IPAddr)
	assert.Equal(t, pb.WifiConnectionResult_WIFI_SUCCESS, result.ResultCode)
}

func TestJoinNetworkTimeout(t *testing.T) {
	cam := newMockCamera(t)
	defer cam.close()

	addr := cam.ln.Addr().(*net.TCPAddr)

	go func() {
		cam.accept(t)

		// Handle sync.
		syncPayload := cam.read(t)
		cam.write(t, syncPayload)

		// Skip keepalives, find the command.
		for {
			payload := cam.read(t)
			if len(payload) >= 3 && payload[0] == pktTypeKeepalive {
				continue
			}
			if len(payload) >= commandHeaderSize && payload[0] == pktTypeMessage {
				cmdCode := binary.LittleEndian.Uint16(payload[3:5])
				if cmdCode == CmdSetWifiConnectionInfo {
					seq := [3]byte{payload[6], payload[7], payload[8]}

					// Send OK response.
					resp := buildOKResponse(seq)
					cam.write(t, resp)

					// Send timeout result.
					result := &pb.CameraWifiConnectionResult{
						WifiConnectionResult: pb.WifiConnectionResult_WIFI_TIMEOUT,
						WifiConnectionInfo: &pb.WifiConnectionInfo{
							Ssid: "BadNet",
						},
					}
					body, err := proto.Marshal(result)
					require.NoError(t, err)

					notifPkt := buildNotificationPacket(NotifyWifiConnectionResult, body)
					cam.write(t, notifPkt)
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

	result, err := JoinNetwork(ctx, c, "BadNet", "password", 10*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out connecting")
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, pb.WifiConnectionResult_WIFI_TIMEOUT, result.ResultCode)
}

func TestJoinNetworkConnectionDropAfterOK(t *testing.T) {
	cam := newMockCamera(t)
	defer cam.close()

	addr := cam.ln.Addr().(*net.TCPAddr)

	go func() {
		cam.accept(t)

		// Handle sync.
		syncPayload := cam.read(t)
		cam.write(t, syncPayload)

		// Skip keepalives, find the command.
		for {
			payload := cam.read(t)
			if len(payload) >= 3 && payload[0] == pktTypeKeepalive {
				continue
			}
			if len(payload) >= commandHeaderSize && payload[0] == pktTypeMessage {
				cmdCode := binary.LittleEndian.Uint16(payload[3:5])
				if cmdCode == CmdSetWifiConnectionInfo {
					seq := [3]byte{payload[6], payload[7], payload[8]}

					// Send OK response, then close connection (simulating camera
					// switching WiFi mode before sending the notification).
					resp := buildOKResponse(seq)
					cam.write(t, resp)

					// Close immediately — the camera dropped its AP.
					cam.conn.Close()
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

	result, err := JoinNetwork(ctx, c, "MyNet", "pass", 10*time.Second)
	// When the connection drops after OK, we assume success (camera switched WiFi).
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "MyNet", result.SSID)
}

func TestGetWifiInfo(t *testing.T) {
	cam := newMockCamera(t)
	defer cam.close()

	addr := cam.ln.Addr().(*net.TCPAddr)

	go func() {
		cam.accept(t)

		// Handle sync.
		syncPayload := cam.read(t)
		cam.write(t, syncPayload)

		// Skip keepalives, find the command.
		for {
			payload := cam.read(t)
			if len(payload) >= 3 && payload[0] == pktTypeKeepalive {
				continue
			}
			if len(payload) >= commandHeaderSize && payload[0] == pktTypeMessage {
				cmdCode := binary.LittleEndian.Uint16(payload[3:5])
				if cmdCode == CmdGetWifiConnectionInfo {
					seq := [3]byte{payload[6], payload[7], payload[8]}

					// Send response with WiFi info.
					info := &pb.GetWifiConnectionInfoResp{
						WifiConnectionInfo: &pb.WifiConnectionInfo{
							Ssid:   "HomeWiFi",
							Bssid:  "AA:BB:CC:DD:EE:FF",
							IpAddr: "192.168.1.100",
						},
					}
					body, err := proto.Marshal(info)
					require.NoError(t, err)

					respPkt := buildResponseWithBody(seq, RespOK, body)
					cam.write(t, respPkt)
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

	info, err := GetWifiInfo(ctx, c, 10*time.Second)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "HomeWiFi", info.Ssid)
	assert.Equal(t, "AA:BB:CC:DD:EE:FF", info.Bssid)
	assert.Equal(t, "192.168.1.100", info.IpAddr)
}

func TestParseWifiConnectionResultEmpty(t *testing.T) {
	_, err := parseWifiConnectionResult(nil, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

// buildNotificationPacket creates a MESSAGE packet with a notification code.
func buildNotificationPacket(notifCode uint16, body []byte) []byte {
	pkt := make([]byte, commandHeaderSize+len(body))
	pkt[0] = pktTypeMessage
	pkt[1] = 0x00
	pkt[2] = 0x00
	binary.LittleEndian.PutUint16(pkt[3:5], notifCode)
	pkt[5] = 0x02
	pkt[6] = 0x00 // seq 0 for notifications
	pkt[7] = 0x00
	pkt[8] = 0x00
	pkt[9] = 0x80
	pkt[10] = 0x00
	pkt[11] = 0x00
	copy(pkt[commandHeaderSize:], body)
	return pkt
}

// buildResponseWithBody creates a MESSAGE response with a specific code and body.
func buildResponseWithBody(seq [3]byte, code uint16, body []byte) []byte {
	pkt := make([]byte, commandHeaderSize+len(body))
	pkt[0] = pktTypeMessage
	pkt[1] = 0x00
	pkt[2] = 0x00
	binary.LittleEndian.PutUint16(pkt[3:5], code)
	pkt[5] = 0x02
	pkt[6] = seq[0]
	pkt[7] = seq[1]
	pkt[8] = seq[2]
	pkt[9] = 0x80
	pkt[10] = 0x00
	pkt[11] = 0x00
	copy(pkt[commandHeaderSize:], body)
	return pkt
}
