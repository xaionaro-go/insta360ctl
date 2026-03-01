package main

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v2"
	"github.com/xaionaro-go/insta360ctl/pkg/remote"
)

func cmdRemote() *cli.Command {
	return &cli.Command{
		Name:  "remote",
		Usage: "Architecture A: GPS Remote emulation",
		Flags: commonRemoteFlags(),
		Subcommands: []*cli.Command{
			{
				Name:  "start",
				Usage: "Start GPS Remote server and wait for camera connection",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "camera-id",
						Usage: "Camera ID (6 hex bytes) for Wake-on-BLE (e.g., AABBCCDDEEFF)",
					},
				},
				Action: func(c *cli.Context) error {
					return runOnRemote(c, func(ctx context.Context, srv *remote.Server) error {
						fmt.Println("GPS Remote server running. Press Ctrl+C to stop.")
						<-ctx.Done()
						return nil
					})
				},
			},
			{
				Name:  "shutter",
				Usage: "Send shutter command to connected camera",
				Action: func(c *cli.Context) error {
					return runOnRemote(c, func(ctx context.Context, srv *remote.Server) error {
						fmt.Println("Sending shutter command...")
						if err := srv.Shutter(); err != nil {
							return err
						}
						fmt.Println("Shutter command sent.")
						return errDone
					})
				},
			},
			{
				Name:  "mode",
				Usage: "Cycle camera mode",
				Action: func(c *cli.Context) error {
					return runOnRemote(c, func(ctx context.Context, srv *remote.Server) error {
						fmt.Println("Cycling camera mode...")
						if err := srv.CycleMode(); err != nil {
							return err
						}
						fmt.Println("Mode cycle command sent.")
						return errDone
					})
				},
			},
			{
				Name:  "screen",
				Usage: "Wake camera screen",
				Action: func(c *cli.Context) error {
					return runOnRemote(c, func(ctx context.Context, srv *remote.Server) error {
						fmt.Println("Waking camera screen...")
						if err := srv.WakeScreen(); err != nil {
							return err
						}
						fmt.Println("Screen wake command sent.")
						return errDone
					})
				},
			},
			{
				Name:  "power-off",
				Usage: "Power off camera via remote",
				Action: func(c *cli.Context) error {
					return runOnRemote(c, func(ctx context.Context, srv *remote.Server) error {
						fmt.Println("Powering off camera...")
						if err := srv.PowerOff(); err != nil {
							return err
						}
						fmt.Println("Power off command sent.")
						return errDone
					})
				},
			},
		},
	}
}
