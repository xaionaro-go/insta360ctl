package direct

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/facebookincubator/go-belt/tool/logger"
	"github.com/xaionaro-go/insta360ctl/pkg/ble"
	"github.com/xaionaro-go/insta360ctl/pkg/protocol"
	"github.com/xaionaro-go/insta360ctl/pkg/protocol/messagecode"
)

// Characteristic UUID prefixes (from the standard/big-endian String() representation).
const (
	// Custom 128-bit UUID service for GO 3.
	customServiceUUIDPrefix = "9e5d1e47"

	// E3DD50BF... — write + notify + indicate (command channel)
	customCharCmdPrefix = "e3dd50bf"
	// 92E86C7A... — write only
	customCharWriteOnlyPrefix = "92e86c7a"
	// 347F7608... — read only
	customCharReadOnlyPrefix = "347f7608"
)

// ProbeCustom performs a focused probe of the Insta360 GO 3's custom BLE service.
//
// It discovers the custom 128-bit UUID service, subscribes to notifications on the
// command characteristic, reads the read-only characteristic, and systematically
// writes single-byte values to both writable characteristics while logging all
// responses. Finally, it attempts Go2BlePacket commands on the standard BE81/B001
// characteristics to check if the custom service "unlocked" the standard protocol.
func (d *Device) ProbeCustom(ctx context.Context) error {
	// Step 0: Find the custom service among discovered services.
	var customSvc ble.Service
	for _, svc := range d.services {
		if strings.HasPrefix(svc.UUID(), customServiceUUIDPrefix) {
			customSvc = svc
			break
		}
	}
	if customSvc == nil {
		return fmt.Errorf("custom service not found (expected UUID prefix %s)", customServiceUUIDPrefix)
	}
	logger.Infof(ctx, "found custom service: uuid=%s", customSvc.UUID())

	// Discover characteristics within the custom service.
	chars, err := customSvc.DiscoverCharacteristics(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover characteristics for custom service: %w", err)
	}

	var charCmd ble.Characteristic      // write + notify + indicate
	var charWriteOnly ble.Characteristic // write only
	var charReadOnly ble.Characteristic  // read only

	for _, c := range chars {
		uuidStr := c.UUID()
		props := c.Properties()
		logger.Infof(ctx, "  custom char: uuid=%s props=0x%02X", uuidStr, props)

		switch {
		case strings.HasPrefix(uuidStr, customCharCmdPrefix):
			charCmd = c
		case strings.HasPrefix(uuidStr, customCharWriteOnlyPrefix):
			charWriteOnly = c
		case strings.HasPrefix(uuidStr, customCharReadOnlyPrefix):
			charReadOnly = c
		}
	}

	if charCmd == nil {
		return fmt.Errorf("command characteristic (prefix %s) not found", customCharCmdPrefix)
	}
	if charWriteOnly == nil {
		return fmt.Errorf("write-only characteristic (prefix %s) not found", customCharWriteOnlyPrefix)
	}
	if charReadOnly == nil {
		return fmt.Errorf("read-only characteristic (prefix %s) not found", customCharReadOnlyPrefix)
	}

	logger.Infof(ctx, "custom char CMD:        uuid=%s", charCmd.UUID())
	logger.Infof(ctx, "custom char WRITE-ONLY: uuid=%s", charWriteOnly.UUID())
	logger.Infof(ctx, "custom char READ-ONLY:  uuid=%s", charReadOnly.UUID())

	// Step 1: Subscribe to notifications/indications on the command characteristic.
	notifCh := make(chan []byte, 64)
	err = charCmd.EnableNotifications(func(b []byte) {
		logger.Infof(context.Background(), "CUSTOM NOTIFICATION: data=%X", b)
		fmt.Printf("CUSTOM NOTIFICATION: data=%X\n", b)
		select {
		case notifCh <- b:
		default:
		}
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to command characteristic notifications: %w", err)
	}
	logger.Infof(ctx, "subscribed to notifications on command characteristic")

	// Helper: drain notifications for a given duration.
	drainNotifications := func(wait time.Duration) {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		for {
			select {
			case data := <-notifCh:
				logger.Infof(ctx, "  -> notification: %X", data)
				fmt.Printf("  -> notification: %X\n", data)
			case <-timer.C:
				return
			case <-ctx.Done():
				return
			}
		}
	}

	// Step 2: Read the read-only characteristic before any writes.
	logger.Infof(ctx, "reading read-only characteristic [BEFORE]...")
	readVal, err := charReadOnly.Read()
	if err != nil {
		logger.Warnf(ctx, "failed to read read-only characteristic: %v", err)
		fmt.Printf("READ-ONLY [BEFORE]: error: %v\n", err)
	} else {
		logger.Infof(ctx, "READ-ONLY [BEFORE]: data=%X", readVal)
		fmt.Printf("READ-ONLY [BEFORE]: data=%X\n", readVal)
	}

	// Step 3: Write 0x01 to the command characteristic (wake-up), wait 1 second.
	logger.Infof(ctx, "writing 0x01 to command characteristic (wake-up)...")
	fmt.Println("writing 0x01 to command characteristic (wake-up)...")
	if err := charCmd.Write([]byte{0x01}, true); err != nil {
		logger.Warnf(ctx, "write 0x01 failed: %v", err)
		fmt.Printf("write 0x01 failed: %v\n", err)
	}
	drainNotifications(1 * time.Second)

	// Step 4: Read the read-only characteristic again to see if it changed.
	logger.Infof(ctx, "reading read-only characteristic [AFTER 0x01]...")
	readVal2, err := charReadOnly.Read()
	if err != nil {
		logger.Warnf(ctx, "failed to read read-only characteristic: %v", err)
		fmt.Printf("READ-ONLY [AFTER 0x01]: error: %v\n", err)
	} else {
		logger.Infof(ctx, "READ-ONLY [AFTER 0x01]: data=%X", readVal2)
		fmt.Printf("READ-ONLY [AFTER 0x01]: data=%X\n", readVal2)
	}

	// Step 5: Scan raw bytes 0x00 through 0x20 on the command characteristic.
	logger.Infof(ctx, "starting raw byte scan (0x00-0x20) on command characteristic...")
	fmt.Println("\n--- Raw byte scan on command characteristic (0x00-0x20) ---")
	for b := byte(0x00); b <= 0x20; b++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		logger.Infof(ctx, "writing 0x%02X to command char...", b)
		fmt.Printf("write 0x%02X: ", b)
		if err := charCmd.Write([]byte{b}, true); err != nil {
			logger.Warnf(ctx, "write 0x%02X failed: %v", b, err)
			fmt.Printf("FAILED (%v)\n", err)
		} else {
			fmt.Printf("OK")
		}
		drainNotifications(500 * time.Millisecond)
		fmt.Println()
	}

	// Step 6: Write bytes 0x00-0x05 to the write-only characteristic.
	logger.Infof(ctx, "starting raw byte scan (0x00-0x05) on write-only characteristic...")
	fmt.Println("\n--- Raw byte scan on write-only characteristic (0x00-0x05) ---")
	for b := byte(0x00); b <= 0x05; b++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		logger.Infof(ctx, "writing 0x%02X to write-only char...", b)
		fmt.Printf("write 0x%02X: ", b)
		if err := charWriteOnly.Write([]byte{b}, true); err != nil {
			logger.Warnf(ctx, "write 0x%02X to write-only failed: %v", b, err)
			fmt.Printf("FAILED (%v)\n", err)
		} else {
			fmt.Printf("OK")
		}
		drainNotifications(500 * time.Millisecond)
		fmt.Println()
	}

	// Step 7: Try Go2BlePacket commands on BE81/B001 to see if the custom char
	// "unlocked" the standard protocol.
	logger.Infof(ctx, "trying Go2BlePacket commands on standard characteristics...")
	fmt.Println("\n--- Go2BlePacket probe on BE81/B001 ---")

	if d.charWrite == nil {
		logger.Warnf(ctx, "no write characteristic available for Go2BlePacket probe")
	} else {
		// First send sync handshake.
		syncData := make([]byte, 7)
		syncPacket := protocol.EncodeGo2BleSyncPacket(syncData)
		logger.Infof(ctx, "sending sync packet: %X", syncPacket)
		fmt.Printf("sync: ")
		if err := d.charWrite.Write(syncPacket, false); err != nil {
			logger.Warnf(ctx, "sync write failed: %v", err)
			fmt.Printf("FAILED (%v)\n", err)
		} else {
			fmt.Printf("OK")
		}
		drainNotifications(500 * time.Millisecond)
		fmt.Println()

		// Then try Go2BlePacket-wrapped commands.
		go2Cmds := []struct {
			name string
			code messagecode.Code
		}{
			{"enter-control", 0x01},
			{"take-photo", messagecode.CodeTakePhoto},
			{"get-battery", messagecode.CodeGetBatteryInfo},
			{"get-device-info", messagecode.CodeGetDeviceInfo},
			{"check-auth", messagecode.CodeCheckAuthorization},
			{"get-camera-state", messagecode.CodeGetCameraState},
		}

		for _, cmd := range go2Cmds {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			innerMsg := protocol.EncodeGo2Message(cmd.code, 0, nil)
			packet := protocol.EncodeGo2BleMessagePacket(innerMsg)
			label := fmt.Sprintf("cmd=%s(0x%02X)", cmd.name, byte(cmd.code))
			logger.Infof(ctx, "Go2BLE [WriteNR] %s data=%X", label, packet)
			fmt.Printf("Go2BLE %s: ", label)
			if err := d.charWrite.Write(packet, false); err != nil {
				logger.Warnf(ctx, "Go2BLE write failed: %v", err)
				fmt.Printf("FAILED (%v)\n", err)
			} else {
				fmt.Printf("OK")
			}
			drainNotifications(500 * time.Millisecond)
			fmt.Println()
		}
	}

	fmt.Println("\nProbeCustom complete.")
	return nil
}
