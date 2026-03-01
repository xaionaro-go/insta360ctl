package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/facebookincubator/go-belt/tool/logger"
	"github.com/urfave/cli/v2"
	"github.com/xaionaro-go/insta360ctl/pkg/wifi"
)

func cmdWifi() *cli.Command {
	return &cli.Command{
		Name:  "wifi",
		Usage: "Configure camera WiFi settings",
		Subcommands: []*cli.Command{
			cmdWifiJoin(),
			cmdWifiInfo(),
		},
	}
}

func commonWifiFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  "addr",
			Value: wifi.DefaultAddr,
			Usage: "Camera IP address (on its AP network)",
		},
		&cli.IntFlag{
			Name:  "port",
			Value: wifi.DefaultPort,
			Usage: "Camera TCP port",
		},
		&cli.DurationFlag{
			Name:  "timeout",
			Value: 60 * time.Second,
			Usage: "Operation timeout",
		},
	}
}

func cmdWifiJoin() *cli.Command {
	return &cli.Command{
		Name:  "join",
		Usage: "Tell the camera to join a WiFi network [not tested, yet]",
		Description: `Sends WiFi credentials to the camera so it joins an existing network
instead of running its own access point. This allows streaming without
a dedicated WiFi adapter — the camera and computer share the same LAN.

You must first connect to the camera's WiFi AP manually. After the
camera joins the target network, it will drop its AP and appear on
the target network with a new IP address.

Flow:
  1. Connect your computer to the camera's WiFi AP
  2. Run: insta360ctl wifi join --ssid "MyNetwork" --password "secret"
  3. Camera joins "MyNetwork" and reports its new IP
  4. Reconnect your computer to "MyNetwork"
  5. Run: insta360ctl stream --addr <camera-ip>

Example:
  insta360ctl wifi join --ssid "HomeWiFi" --password "hunter2"`,
		Flags: append(commonWifiFlags(),
			&cli.StringFlag{
				Name:     "ssid",
				Required: true,
				Usage:    "Target WiFi network name",
			},
			&cli.StringFlag{
				Name:     "password",
				Required: true,
				Usage:    "Target WiFi network password",
			},
		),
		Action: func(c *cli.Context) error {
			return runWifiJoin(c)
		},
	}
}

func cmdWifiInfo() *cli.Command {
	return &cli.Command{
		Name:  "info",
		Usage: "Query the camera's current WiFi connection info [not tested, yet]",
		Flags: commonWifiFlags(),
		Action: func(c *cli.Context) error {
			return runWifiInfo(c)
		},
	}
}

func runWifiJoin(c *cli.Context) error {
	loggerLevel, err := getLoggerLevel(c)
	if err != nil {
		return err
	}

	ctx := getContext(loggerLevel, false)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	addr := c.String("addr")
	port := c.Int("port")
	timeout := c.Duration("timeout")
	ssid := c.String("ssid")
	password := c.String("password")

	conn, err := wifi.Dial(ctx, addr, port)
	if err != nil {
		return fmt.Errorf("failed to connect to camera: %w", err)
	}
	defer conn.Close()

	logger.Infof(ctx, "telling camera to join WiFi network %q...", ssid)

	result, err := wifi.JoinNetwork(ctx, conn, ssid, password, timeout)
	if err != nil {
		if result != nil {
			// Partial result — print what we know.
			fmt.Fprintf(os.Stderr, "WiFi join failed: %v\n", err)
			fmt.Fprintf(os.Stderr, "SSID: %s\n", result.SSID)
			fmt.Fprintf(os.Stderr, "Result: %s\n", result.ResultCode)
		}
		return err
	}

	fmt.Printf("Camera joined WiFi network %q successfully.\n", result.SSID)
	if result.IPAddr != "" {
		fmt.Printf("Camera IP: %s\n", result.IPAddr)
		fmt.Printf("\nTo stream: insta360ctl stream --addr %s\n", result.IPAddr)
	} else {
		fmt.Printf("Camera IP not reported. Check your router's DHCP leases.\n")
		fmt.Printf("\nTo stream: insta360ctl stream --addr <camera-ip>\n")
	}

	return nil
}

func runWifiInfo(c *cli.Context) error {
	loggerLevel, err := getLoggerLevel(c)
	if err != nil {
		return err
	}

	ctx := getContext(loggerLevel, false)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	addr := c.String("addr")
	port := c.Int("port")
	timeout := c.Duration("timeout")

	conn, err := wifi.Dial(ctx, addr, port)
	if err != nil {
		return fmt.Errorf("failed to connect to camera: %w", err)
	}
	defer conn.Close()

	info, err := wifi.GetWifiInfo(ctx, conn, timeout)
	if err != nil {
		return err
	}

	if info == nil {
		fmt.Println("No WiFi connection info available.")
		return nil
	}

	fmt.Printf("SSID:     %s\n", info.Ssid)
	fmt.Printf("BSSID:    %s\n", info.Bssid)
	fmt.Printf("IP:       %s\n", info.IpAddr)

	return nil
}
