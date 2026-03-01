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
	pb "github.com/xaionaro-go/insta360ctl/pkg/protocol/protobuf"
	"github.com/xaionaro-go/insta360ctl/pkg/wifi"
)

func cmdStream() *cli.Command {
	return &cli.Command{
		Name:  "stream",
		Usage: "Get live video preview from the camera over WiFi [not tested, yet]",
		Description: `Connects to the camera's WiFi TCP protocol (port 6666) and streams
live video preview data. The video is output as raw H.264/H.265 NAL units.

You must first connect to the camera's WiFi access point manually,
OR use --wifi-ssid/--wifi-password to make the camera join your LAN.

The camera creates an AP with SSID like "GO 3 XXXX" or "X3 XXXX"
(default password: 88888888).

Examples:
  # Stream to stdout, pipe to mpv for playback
  insta360ctl stream | mpv --no-correct-pts --fps=30 -

  # Save raw video to file
  insta360ctl stream --output preview.h265

  # Use specific camera IP and resolution
  insta360ctl stream --addr 192.168.42.1 --resolution 1920x1080

  # First make camera join your LAN, then stream
  insta360ctl stream --wifi-ssid "HomeWiFi" --wifi-password "secret" --output preview.h265`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "addr",
				Value: wifi.DefaultAddr,
				Usage: "Camera IP address",
			},
			&cli.IntFlag{
				Name:  "port",
				Value: wifi.DefaultPort,
				Usage: "Camera TCP port",
			},
			&cli.StringFlag{
				Name:  "resolution",
				Value: "1440x720",
				Usage: "Stream resolution: 1440x720, 1920x1080, 424x240",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Value:   "",
				Usage:   "Output file (default: stdout)",
			},
			&cli.BoolFlag{
				Name:  "gyro",
				Value: false,
				Usage: "Enable gyro/IMU data (logged, not written to output)",
			},
			&cli.DurationFlag{
				Name:  "duration",
				Value: 0,
				Usage: "Maximum stream duration (0 = unlimited)",
			},
			&cli.IntFlag{
				Name:  "bitrate",
				Value: 40,
				Usage: "Video bitrate parameter",
			},
			&cli.StringFlag{
				Name:  "wifi-ssid",
				Value: "",
				Usage: "Tell camera to join this WiFi network before streaming",
			},
			&cli.StringFlag{
				Name:  "wifi-password",
				Value: "",
				Usage: "Password for the --wifi-ssid network",
			},
			&cli.DurationFlag{
				Name:  "wifi-timeout",
				Value: 30 * time.Second,
				Usage: "Timeout for WiFi join operation",
			},
		},
		Action: func(c *cli.Context) error {
			return runStream(c)
		},
	}
}

func parseResolution(s string) pb.VideoResolution {
	switch s {
	case "1440x720", "720p":
		return pb.VideoResolution_RES_1440_720P30
	case "1920x1080", "1080p":
		return pb.VideoResolution_RES_1920_1080P30
	case "424x240", "240p":
		return pb.VideoResolution_RES_424_240P15
	case "480x240":
		return pb.VideoResolution_RES_480_240P30
	default:
		return pb.VideoResolution_RES_1440_720P30
	}
}

func runStream(c *cli.Context) error {
	loggerLevel, err := getLoggerLevel(c)
	if err != nil {
		return err
	}

	ctx := getContext(loggerLevel, false)

	// Handle Ctrl+C for graceful shutdown.
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

	if duration := c.Duration("duration"); duration > 0 {
		var dCancel context.CancelFunc
		ctx, dCancel = context.WithTimeout(ctx, duration)
		defer dCancel()
	}

	addr := c.String("addr")
	port := c.Int("port")

	// If --wifi-ssid is set, first tell the camera to join that network.
	if wifiSSID := c.String("wifi-ssid"); wifiSSID != "" {
		newAddr, err := joinWifiThenReconnect(ctx, c, addr, port, wifiSSID)
		if err != nil {
			return err
		}
		addr = newAddr
	}

	// Connect to camera.
	conn, err := wifi.Dial(ctx, addr, port)
	if err != nil {
		return fmt.Errorf("failed to connect to camera: %w", err)
	}
	defer conn.Close()

	// Open output.
	var output *os.File
	outputPath := c.String("output")
	if outputPath != "" {
		output, err = os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer output.Close()
		logger.Infof(ctx, "writing video to %s", outputPath)
	} else {
		output = os.Stdout
	}

	// Build stream options.
	opts := wifi.StreamOptions{
		Resolution:          parseResolution(c.String("resolution")),
		SecondaryResolution: pb.VideoResolution_RES_424_240P15,
		VideoBitrate:        uint32(c.Int("bitrate")),
		EnableGyro:          c.Bool("gyro"),
	}

	// Start streaming.
	streamer := wifi.NewStreamer(conn)
	if err := streamer.Start(ctx, opts); err != nil {
		return fmt.Errorf("start stream: %w", err)
	}

	logger.Infof(ctx, "streaming %s video (Ctrl+C to stop)...", c.String("resolution"))

	// If gyro enabled, log gyro packets in background.
	if opts.EnableGyro {
		go func() {
			for pkt := range streamer.GyroCh {
				logger.Debugf(ctx, "gyro: ts=%d len=%d", pkt.Timestamp, len(pkt.Data))
			}
		}()
	}

	// Write video data until context cancels.
	err = streamer.WriteVideoTo(ctx, output)

	// Send stop command (best effort).
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if stopErr := streamer.Stop(stopCtx); stopErr != nil {
		logger.Warnf(ctx, "failed to send STOP_LIVE_STREAM: %v", stopErr)
	}

	if err != nil && ctx.Err() != nil {
		// Context cancelled (Ctrl+C or duration expired) — not an error.
		logger.Infof(ctx, "stream ended")
		return nil
	}

	return err
}

// joinWifiThenReconnect tells the camera to join a WiFi network, waits for
// the result, and returns the camera's new IP address on the target network.
func joinWifiThenReconnect(ctx context.Context, c *cli.Context, addr string, port int, wifiSSID string) (string, error) {
	wifiPassword := c.String("wifi-password")
	wifiTimeout := c.Duration("wifi-timeout")

	logger.Infof(ctx, "connecting to camera AP to send WiFi join command...")

	apConn, err := wifi.Dial(ctx, addr, port)
	if err != nil {
		return "", fmt.Errorf("failed to connect to camera AP: %w", err)
	}

	logger.Infof(ctx, "telling camera to join %q...", wifiSSID)

	result, err := wifi.JoinNetwork(ctx, apConn, wifiSSID, wifiPassword, wifiTimeout)
	apConn.Close()

	if err != nil {
		return "", fmt.Errorf("WiFi join failed: %w", err)
	}

	if result.IPAddr == "" {
		return "", fmt.Errorf("camera joined %q but did not report its IP address; check your router's DHCP leases and use --addr", wifiSSID)
	}

	logger.Infof(ctx, "camera joined %q with IP %s", wifiSSID, result.IPAddr)

	// Give the camera a moment to stabilize on the new network.
	select {
	case <-time.After(2 * time.Second):
	case <-ctx.Done():
		return "", ctx.Err()
	}

	return result.IPAddr, nil
}
