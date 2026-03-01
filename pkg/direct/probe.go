package direct

import (
	"context"
	"fmt"
	"time"

	"github.com/facebookincubator/go-belt/tool/logger"
	"github.com/xaionaro-go/insta360ctl/pkg/ble"
	"github.com/xaionaro-go/insta360ctl/pkg/protocol"
	"github.com/xaionaro-go/insta360ctl/pkg/protocol/messagecode"
)

// Probe systematically writes to every writable characteristic with various
// formats and command codes, reporting any notifications received.
// This is used to discover the correct protocol for unknown cameras.
func (d *Device) Probe(ctx context.Context) error {
	type writeTarget struct {
		char ble.Characteristic
		uuid string
	}
	var targets []writeTarget

	// Use characteristics already discovered during Init().
	for _, c := range d.allChars {
		props := c.Properties()
		uid := c.UUID()

		if props&(ble.CharPropWrite|ble.CharPropWriteNR) != 0 {
			targets = append(targets, writeTarget{char: c, uuid: uid})
		}

		// Subscribe to any notify/indicate characteristics we haven't subscribed to yet.
		if props&(ble.CharPropNotify|ble.CharPropIndicate) != 0 {
			ntUUID := uid
			err := c.EnableNotifications(func(b []byte) {
				logger.Infof(context.Background(), "PROBE notification: uuid=%s data=%X", ntUUID, b)
				fmt.Printf("NOTIFICATION: uuid=%s data=%X\n", ntUUID, b)
			})
			if err != nil {
				logger.Debugf(ctx, "  subscribe %s failed: %v (may already be subscribed)", uid, err)
			} else {
				logger.Infof(ctx, "  subscribed to %s", uid)
			}
		}
	}

	logger.Infof(ctx, "found %d writable characteristics", len(targets))

	// Commands to try.
	cmds := []struct {
		name string
		code byte
	}{
		{"enter-control", 0x01},
		{"take-photo", 0x03},
		{"status", 0x08},
		{"get-device-info", 0x28},
		{"x3-status", 0x42},
	}

	// Formats to try.
	type format struct {
		name   string
		encode func(cmd byte) []byte
	}
	formats := []format{
		{"go2ble", func(cmd byte) []byte {
			// Correct Go2BlePacket format: inner Go2 message wrapped in BLE envelope.
			innerMsg := protocol.EncodeGo2Message(messagecode.Code(cmd), 0, nil)
			return protocol.EncodeGo2BleMessagePacket(innerMsg)
		}},
		{"header16", func(cmd byte) []byte {
			return protocol.EncodeMessage(messagecode.Code(cmd), 0, nil)
		}},
		{"ff-frame-17", func(cmd byte) []byte {
			return protocol.EncodeFFFrame(0x06, cmd, nil, 17)
		}},
		{"ff-frame-8", func(cmd byte) []byte {
			return protocol.EncodeFFFrame(0x09, cmd, []byte{0x01, 0x00}, 0)
		}},
		{"raw-1byte", func(cmd byte) []byte {
			return []byte{cmd}
		}},
	}

	// Drain any initial notifications.
	time.Sleep(500 * time.Millisecond)

	logger.Infof(ctx, "starting probe: %d chars x %d formats x %d cmds x 2 write-modes = %d attempts",
		len(targets), len(formats), len(cmds), len(targets)*len(formats)*len(cmds)*2)

	for _, tgt := range targets {
		for _, f := range formats {
			for _, cmd := range cmds {
				if ctx.Err() != nil {
					return ctx.Err()
				}

				msg := f.encode(cmd.code)
				label := fmt.Sprintf("char=%s fmt=%s cmd=%s(0x%02X)", tgt.uuid, f.name, cmd.name, cmd.code)

				// Try WriteWithoutResponse.
				logger.Infof(ctx, "PROBE [WriteNR] %s data=%X", label, msg)
				err := tgt.char.Write(msg, false)
				if err != nil {
					logger.Debugf(ctx, "  write-nr failed: %v", err)
				}

				// Wait briefly for a response.
				time.Sleep(200 * time.Millisecond)

				// Also try WriteWithResponse.
				logger.Infof(ctx, "PROBE [WriteReq] %s data=%X", label, msg)
				err = tgt.char.Write(msg, true)
				if err != nil {
					logger.Debugf(ctx, "  write-req failed: %v", err)
					continue
				}

				time.Sleep(200 * time.Millisecond)
			}
		}
	}

	fmt.Println("\nProbe complete. No matches means the protocol is unknown or needs authentication.")
	return nil
}
