package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/facebookincubator/go-belt/tool/logger"
	"github.com/urfave/cli/v2"
	"github.com/xaionaro-go/insta360ctl/pkg/ble"
	"github.com/xaionaro-go/insta360ctl/pkg/direct"
	"github.com/xaionaro-go/insta360ctl/pkg/remote"
)

var errDone = fmt.Errorf("done")

func getLoggerLevel(c *cli.Context) (logger.Level, error) {
	var loggerLevel logger.Level
	if err := loggerLevel.Set(c.String("log-level")); err != nil {
		return 0, fmt.Errorf("invalid log level '%s': %w", c.String("log-level"), err)
	}
	return loggerLevel, nil
}

// newAdapter creates a BLE adapter based on the --ble-backend flag.
func newAdapter(ctx context.Context, c *cli.Context) (ble.Adapter, error) {
	backend := c.String("ble-backend")
	adapterID := c.String("ble-adapter")
	switch backend {
	case "dbus":
		return ble.NewDBusAdapter(ctx, adapterID)
	case "hci":
		return ble.NewHCIAdapter(ctx)
	default:
		return nil, fmt.Errorf("unknown BLE backend '%s' (valid: dbus, hci)", backend)
	}
}

// runOnDirect scans for an Insta360 camera, connects via Architecture B,
// initializes the device, and executes the given action.
func runOnDirect(c *cli.Context, action func(ctx context.Context, dev *direct.Device) error) error {
	loggerLevel, err := getLoggerLevel(c)
	if err != nil {
		return err
	}

	ctx := getContext(loggerLevel, false)
	filterAddr := c.String("addr")
	timeout := c.Duration("timeout")

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	adapter, err := newAdapter(ctx, c)
	if err != nil {
		return fmt.Errorf("failed to create BLE adapter: %w", err)
	}
	defer adapter.Close(ctx)

	devCh, errCh := direct.Scan(ctx, adapter)

	for {
		select {
		case dev := <-devCh:
			if filterAddr != "" && !strings.Contains(strings.ToLower(dev.ID.String()), strings.ToLower(filterAddr)) {
				logger.Infof(ctx, "found %s; skipping (address filter: '%s')", dev, filterAddr)
				continue
			}

			if err := dev.Init(ctx); err != nil {
				return fmt.Errorf("failed to initialize: %w", err)
			}

			defer func() { _ = dev.Close(ctx) }()

			if err := action(ctx, dev); err != nil {
				if err == errDone {
					return nil
				}
				return err
			}
			return nil

		case err := <-errCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// runOnRemote sets up a GPS Remote server (Architecture A), starts advertising,
// waits for a camera to connect, then executes the action.
// Note: Remote mode requires the HCI backend since it needs GATT server support.
func runOnRemote(c *cli.Context, action func(ctx context.Context, srv *remote.Server) error) error {
	loggerLevel, err := getLoggerLevel(c)
	if err != nil {
		return err
	}

	ctx := getContext(loggerLevel, false)
	timeout := c.Duration("timeout")

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	adapter, err := ble.NewHCIAdapter(ctx)
	if err != nil {
		return fmt.Errorf("failed to create HCI adapter (remote mode requires HCI): %w", err)
	}

	srv := remote.NewServer(adapter.GattDevice(), "")

	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("failed to start remote server: %w", err)
	}

	if err := srv.Advertise(ctx); err != nil {
		return fmt.Errorf("failed to advertise: %w", err)
	}

	logger.Infof(ctx, "GPS Remote server started, waiting for camera connection...")

	if err := action(ctx, srv); err != nil {
		if err == errDone {
			return nil
		}
		return err
	}

	return nil
}
