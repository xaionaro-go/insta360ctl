package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/urfave/cli/v2"
	"github.com/xaionaro-go/insta360ctl/pkg/camera"
	"github.com/xaionaro-go/insta360ctl/pkg/direct"
	"github.com/xaionaro-go/insta360ctl/pkg/protocol/messagecode"
	pb "github.com/xaionaro-go/insta360ctl/pkg/protocol/protobuf"
)

func cmdDirect() *cli.Command {
	return &cli.Command{
		Name:  "direct",
		Usage: "Architecture B: Direct camera control via BLE",
		Flags: commonDirectFlags(),
		Subcommands: []*cli.Command{
			{
				Name:  "shutter",
				Usage: "Trigger the camera shutter (photo or start/stop recording) [tested: GO 3]",
				Action: func(c *cli.Context) error {
					return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
						return dev.TakePhoto(ctx)
					})
				},
			},
			{
				Name:  "photo",
				Usage: "Take a photo [tested: GO 3]",
				Action: func(c *cli.Context) error {
					return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
						if err := dev.SetMode(ctx, camera.CaptureModePhoto); err != nil {
							return err
						}
						return dev.TakePhoto(ctx)
					})
				},
			},
			{
				Name:  "record",
				Usage: "Start or stop video recording",
				Subcommands: []*cli.Command{
					{
						Name:  "start",
						Usage: "Start recording [tested: GO 3]",
						Action: func(c *cli.Context) error {
							return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
								return dev.StartRecording(ctx)
							})
						},
					},
					{
						Name:  "stop",
						Usage: "Stop recording [tested: GO 3]",
						Action: func(c *cli.Context) error {
							return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
								return dev.StopRecording(ctx)
							})
						},
					},
				},
			},
			{
				Name:      "mode",
				Usage:     "Set camera capture mode [tested: GO 3]",
				ArgsUsage: "<photo|video|timelapse|hdr|bullettime>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("mode argument required (photo, video, timelapse, hdr, bullettime)")
					}
					mode, ok := camera.ParseCaptureMode(c.Args().First())
					if !ok {
						return fmt.Errorf("invalid mode: %s", c.Args().First())
					}
					return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
						return dev.SetMode(ctx, mode)
					})
				},
			},
			{
				Name:  "status",
				Usage: "Show camera status (battery, storage) [tested: GO 3]",
				Action: func(c *cli.Context) error {
					return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
						battery, err := dev.GetBatteryInfo(ctx)
						if err != nil {
							fmt.Printf("Battery: unavailable (%v)\n", err)
						} else {
							fmt.Printf("Battery: %d%%", battery.Level)
							if battery.Charging {
								fmt.Printf(" (charging)")
							}
							if battery.Voltage > 0 {
								fmt.Printf(" (%dmV)", battery.Voltage)
							}
							fmt.Println()
						}

						storage, err := dev.GetStorageInfo(ctx)
						if err != nil {
							fmt.Printf("Storage: unavailable (%v)\n", err)
						} else {
							fmt.Printf("Storage: %dMB free / %dMB total (%d files)\n",
								storage.FreeMB, storage.TotalMB, storage.FileCount)
						}
						return errDone
					})
				},
			},
			{
				Name:  "battery",
				Usage: "Show battery level [tested: GO 3]",
				Action: func(c *cli.Context) error {
					return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
						info, err := dev.GetBatteryInfo(ctx)
						if err != nil {
							return err
						}
						fmt.Printf("Battery: %d%%\n", info.Level)
						if info.Charging {
							fmt.Println("Charging: yes")
						}
						if info.Voltage > 0 {
							fmt.Printf("Voltage: %dmV\n", info.Voltage)
						}
						return errDone
					})
				},
			},
			{
				Name:  "storage",
				Usage: "Show storage info [tested: GO 3]",
				Action: func(c *cli.Context) error {
					return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
						info, err := dev.GetStorageInfo(ctx)
						if err != nil {
							return err
						}
						fmt.Printf("Total:  %d MB\n", info.TotalMB)
						fmt.Printf("Free:   %d MB\n", info.FreeMB)
						fmt.Printf("Files:  %d\n", info.FileCount)
						return errDone
					})
				},
			},
			{
				Name:      "gps",
				Usage:     "Inject GPS coordinates for geotagging [tested: GO 3]",
				ArgsUsage: "<latitude> <longitude> <altitude>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 3 {
						return fmt.Errorf("usage: gps <latitude> <longitude> <altitude>")
					}
					lat, err := strconv.ParseFloat(c.Args().Get(0), 64)
					if err != nil {
						return fmt.Errorf("invalid latitude: %w", err)
					}
					lon, err := strconv.ParseFloat(c.Args().Get(1), 64)
					if err != nil {
						return fmt.Errorf("invalid longitude: %w", err)
					}
					alt, err := strconv.ParseFloat(c.Args().Get(2), 64)
					if err != nil {
						return fmt.Errorf("invalid altitude: %w", err)
					}
					return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
						return dev.InjectGPS(ctx, lat, lon, alt)
					})
				},
			},
			{
				Name:  "marker",
				Usage: "Set a highlight marker at the current recording position [not tested, yet]",
				Action: func(c *cli.Context) error {
					return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
						return dev.SetHighlight(ctx)
					})
				},
			},
			{
				Name:  "power-off",
				Usage: "Power off the camera [not tested, yet]",
				Action: func(c *cli.Context) error {
					return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
						fmt.Println("Powering off camera...")
						return dev.PowerOff(ctx)
					})
				},
			},
			{
				Name:  "info",
				Usage: "Show device information (model, firmware, serial) [not tested, yet]",
				Action: func(c *cli.Context) error {
					return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
						fmt.Printf("Name:   %s\n", dev.Name())
						fmt.Printf("Model:  %s\n", dev.Model())
						fmt.Printf("Addr:   %s\n", dev.Address)

						fw, sn, err := dev.GetDeviceInfo(ctx)
						if err != nil {
							fmt.Printf("Info:   unavailable (%v)\n", err)
						} else {
							if fw != "" {
								fmt.Printf("FW:     %s\n", fw)
							}
							if sn != "" {
								fmt.Printf("Serial: %s\n", sn)
							}
						}
						return errDone
					})
				},
			},
			{
				Name:  "probe",
				Usage: "Systematically probe all writable characteristics with various formats and commands",
				Flags: []cli.Flag{
					&cli.DurationFlag{
						Name:  "timeout",
						Value: 5 * time.Minute,
						Usage: "Probe timeout (overrides parent --timeout)",
					},
				},
				Action: func(c *cli.Context) error {
					return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
						return dev.Probe(ctx)
					})
				},
			},
			{
				Name:  "probe-custom",
				Usage: "Probe the custom 128-bit UUID service (Insta360 GO 3 specific)",
				Flags: []cli.Flag{
					&cli.DurationFlag{
						Name:  "timeout",
						Value: 5 * time.Minute,
						Usage: "Probe timeout (overrides parent --timeout)",
					},
				},
				Action: func(c *cli.Context) error {
					return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
						return dev.ProbeCustom(ctx)
					})
				},
			},
			{
				Name:      "raw",
				Usage:     "Send a raw command code (hex) and display the response",
				ArgsUsage: "<cmd-hex> [param-hex...]",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: raw <cmd-hex> [param-hex...]")
					}
					cmdCode, err := strconv.ParseUint(c.Args().Get(0), 16, 16)
					if err != nil {
						return fmt.Errorf("invalid command code: %w", err)
					}
					var params []byte
					for i := 1; i < c.NArg(); i++ {
						b, err := strconv.ParseUint(c.Args().Get(i), 16, 8)
						if err != nil {
							return fmt.Errorf("invalid param byte: %w", err)
						}
						params = append(params, byte(b))
					}
					return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
						resp, err := dev.SendCommand(ctx, messagecode.Code(cmdCode), params)
						if err != nil {
							return fmt.Errorf("command 0x%02X failed: %w", cmdCode, err)
						}
						fmt.Printf("Response: %X\n", resp)
						return errDone
					})
				},
			},
			{
				Name:  "timelapse",
				Usage: "Start or stop timelapse capture",
				Subcommands: []*cli.Command{
					{
						Name:  "start",
						Usage: "Start timelapse [not tested, yet]",
						Action: func(c *cli.Context) error {
							return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
								return dev.StartTimelapse(ctx)
							})
						},
					},
					{
						Name:  "stop",
						Usage: "Stop timelapse [not tested, yet]",
						Action: func(c *cli.Context) error {
							return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
								return dev.StopTimelapse(ctx)
							})
						},
					},
				},
			},
			{
				Name:  "wifi-join",
				Usage: "Tell the camera to join a WiFi network (0x70) [not tested, yet]",
				Description: `Sends WiFi credentials via SET_WIFI_CONNECTION_INFO (command 0x70).

You may need to first switch the camera to STA mode with 'wifi-mode sta'.

Example:
  sudo insta360ctl direct wifi-mode sta
  sudo insta360ctl direct wifi-join --ssid "HomeWiFi" --password "secret"`,
				Flags: []cli.Flag{
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
				},
				Action: func(c *cli.Context) error {
					ssid := c.String("ssid")
					password := c.String("password")
					return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
						fmt.Printf("Sending WiFi credentials for %q to camera (0x70)...\n", ssid)
						result, err := dev.SetWifiConfig(ctx, ssid, password)
						if err != nil {
							if result != nil {
								fmt.Printf("Response: %X\n", result.RawResponse)
							}
							return err
						}
						fmt.Printf("Camera accepted WiFi configuration for %q.\n", ssid)
						fmt.Println("The camera should now attempt to join the network.")
						fmt.Println("Check your router's DHCP leases for the camera's new IP.")
						fmt.Println()
						fmt.Println("To stream: insta360ctl stream --addr <camera-ip>")
						return errDone
					})
				},
			},
			{
				Name:        "wifi-info",
				Usage:       "Query the camera's current WiFi configuration (0x71) [not tested, yet]",
				Description: `Sends GET_WIFI_CONNECTION_INFO (command 0x71).`,
				Action: func(c *cli.Context) error {
					return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
						resp, err := dev.GetWifiConfig(ctx)
						if err != nil {
							return err
						}
						fmt.Printf("WiFi config response: %X\n", resp)
						if len(resp) > 0 {
							fmt.Printf("WiFi config (string): %s\n", string(resp))
						}
						return errDone
					})
				},
			},
			{
				Name:      "wifi-mode",
				Usage:     "Set WiFi operating mode (ap/sta/p2p) (0x93) [not tested, yet]",
				ArgsUsage: "<ap|sta|p2p>",
				Description: `Sets the camera's WiFi mode via SET_WIFI_MODE (command 0x93).

Modes:
  ap  - Camera acts as WiFi access point (default)
  sta - Camera joins an existing WiFi network (station mode)
  p2p - WiFi Direct peer-to-peer

To join a network, switch to STA mode first, then use wifi-join.`,
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: wifi-mode <ap|sta|p2p>")
					}
					var mode pb.WifiMode
					switch c.Args().First() {
					case "ap":
						mode = pb.WifiMode_WIFI_MODE_AP
					case "sta":
						mode = pb.WifiMode_WIFI_MODE_STA
					case "p2p":
						mode = pb.WifiMode_WIFI_MODE_P2P
					default:
						return fmt.Errorf("invalid mode: %s (use ap, sta, or p2p)", c.Args().First())
					}
					return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
						fmt.Printf("Setting WiFi mode to %s...\n", c.Args().First())
						if err := dev.SetWifiMode(ctx, mode); err != nil {
							return err
						}
						fmt.Printf("WiFi mode set to %s.\n", c.Args().First())
						return errDone
					})
				},
			},
			{
				Name:  "wifi-mode-get",
				Usage: "Query current WiFi operating mode (0x96) [not tested, yet]",
				Action: func(c *cli.Context) error {
					return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
						mode, err := dev.GetWifiMode(ctx)
						if err != nil {
							return err
						}
						fmt.Printf("WiFi mode: %s\n", mode)
						return errDone
					})
				},
			},
			{
				Name:  "wifi-scan",
				Usage: "Trigger a WiFi network scan (0x94) [not tested, yet]",
				Action: func(c *cli.Context) error {
					return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
						fmt.Println("Requesting WiFi scan...")
						if err := dev.GetWifiScanList(ctx); err != nil {
							return err
						}
						fmt.Println("Scan requested. Results arrive via camera notification.")
						return errDone
					})
				},
			},
			{
				Name:  "wifi-reset",
				Usage: "Reset WiFi configuration (0x7D) [not tested, yet]",
				Action: func(c *cli.Context) error {
					return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
						fmt.Println("Resetting WiFi configuration...")
						if err := dev.ResetWifi(ctx); err != nil {
							return err
						}
						fmt.Println("WiFi configuration reset.")
						return errDone
					})
				},
			},
			{
				Name:      "hdr",
				Usage:     "Enable or disable HDR mode [not tested, yet]",
				ArgsUsage: "<on|off>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: hdr <on|off>")
					}
					var enable bool
					switch c.Args().First() {
					case "on", "true", "1":
						enable = true
					case "off", "false", "0":
						enable = false
					default:
						return fmt.Errorf("invalid argument: use 'on' or 'off'")
					}
					return runOnDirect(c, func(ctx context.Context, dev *direct.Device) error {
						return dev.SetHDR(ctx, enable)
					})
				},
			},
		},
	}
}
