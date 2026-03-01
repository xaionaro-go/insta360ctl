package main

import (
	"fmt"
	"os"
	"time"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "insta360ctl",
		Usage: "Insta360 camera control tool via BLE and WiFi",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "log-level",
				Value: "info",
				Usage: "Log level (debug, info, warn, error, fatal, panic)",
			},
			&cli.StringFlag{
				Name:  "ble-backend",
				Value: "dbus",
				Usage: "BLE backend: dbus (BlueZ D-Bus) or hci (raw HCI sockets)",
			},
			&cli.StringFlag{
				Name:  "ble-adapter",
				Value: "",
				Usage: "BLE adapter ID (e.g. hci0, hci1); empty = default",
			},
		},
		Commands: []*cli.Command{
			cmdScan(),
			cmdDirect(),
			cmdRemote(),
			cmdStream(),
			cmdWifi(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		if err == errDone {
			return
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func commonDirectFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  "addr",
			Value: "",
			Usage: "Filter camera by BLE MAC address",
		},
		&cli.DurationFlag{
			Name:  "timeout",
			Value: 30 * time.Second,
			Usage: "Connection timeout",
		},
	}
}

func commonRemoteFlags() []cli.Flag {
	return []cli.Flag{
		&cli.DurationFlag{
			Name:  "timeout",
			Value: 60 * time.Second,
			Usage: "Operation timeout",
		},
		&cli.BoolFlag{
			Name:  "wait",
			Value: true,
			Usage: "Wait for camera to connect before sending command",
		},
	}
}
