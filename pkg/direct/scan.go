package direct

import (
	"context"
	"fmt"
	"sync"

	"github.com/facebookincubator/go-belt/tool/logger"
	"github.com/xaionaro-go/insta360ctl/pkg/ble"
	"github.com/xaionaro-go/insta360ctl/pkg/camera"
)

// Scan discovers Insta360 cameras advertising via BLE.
// It returns a channel of discovered devices.
func Scan(ctx context.Context, adapter ble.Adapter) (<-chan *Device, <-chan error) {
	retCh := make(chan *Device, 100)
	errCh := make(chan error, 2)

	seen := sync.Map{}

	go func() {
		defer close(retCh)
		err := adapter.Scan(ctx, func(result ble.ScanResult) {
			model := camera.IdentifyFromBLEName(result.LocalName)
			if model == camera.ModelUnknown {
				return
			}

			if _, loaded := seen.LoadOrStore(result.Address, true); loaded {
				return
			}

			id, err := ble.ParseDeviceID(result.Address)
			if err != nil {
				logger.Warnf(ctx, "unable to parse device ID '%s': %v", result.Address, err)
				return
			}

			dev := &Device{
				Adapter:        adapter,
				ID:             id,
				Cam:            model,
				BLName:         result.LocalName,
				RSSI:           result.RSSI,
				Address:        result.Address,
				session:        NewSession(),
				responseChs:    make(map[uint8]chan []byte),
				notificationCh: make(chan []byte, 64),
			}

			logger.Infof(ctx, "discovered camera: %s", dev)
			select {
			case retCh <- dev:
			case <-ctx.Done():
			}
		})
		if err != nil && ctx.Err() == nil {
			errCh <- fmt.Errorf("scan failed: %w", err)
		}
	}()

	return retCh, errCh
}
