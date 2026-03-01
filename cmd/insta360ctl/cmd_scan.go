package main

import (
	"context"
	"fmt"
	"time"

	"github.com/facebookincubator/go-belt/tool/logger"
	"github.com/urfave/cli/v2"
	"github.com/xaionaro-go/insta360ctl/pkg/direct"
)

func cmdScan() *cli.Command {
	return &cli.Command{
		Name:  "scan",
		Usage: "Scan for Insta360 cameras",
		Flags: []cli.Flag{
			&cli.DurationFlag{
				Name:  "timeout",
				Value: 10 * time.Second,
				Usage: "Scan timeout",
			},
		},
		Action: func(c *cli.Context) error {
			loggerLevel, err := getLoggerLevel(c)
			if err != nil {
				return err
			}

			ctx := getContext(loggerLevel, false)
			timeout := c.Duration("timeout")
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			adapter, err := newAdapter(ctx, c)
			if err != nil {
				return fmt.Errorf("failed to create BLE adapter: %w", err)
			}
			defer adapter.Close(ctx)

			devCh, errCh := direct.Scan(ctx, adapter)

			fmt.Printf("Scanning for Insta360 cameras (%s)...\n", timeout)
			count := 0

			for {
				select {
				case dev := <-devCh:
					count++
					fmt.Printf("  [%d] %s\n", count, dev)
				case err := <-errCh:
					return err
				case <-ctx.Done():
					if count == 0 {
						fmt.Println("No cameras found.")
					} else {
						fmt.Printf("\nFound %d camera(s).\n", count)
					}
					logger.Debugf(ctx, "scan finished: %v", ctx.Err())
					return nil
				}
			}
		},
	}
}
