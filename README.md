# insta360ctl

A Go CLI tool for controlling Insta360 cameras via BLE and WiFi from Linux.

## Supported Cameras

| Camera | Direct Control (B) | GPS Remote (A) |
|--------|-------------------|----------------|
| X3     | Yes               | Yes            |
| X4     | Yes               | Yes            |
| X5     | Yes               | Yes            |
| ONE X3 | Yes               | Yes            |
| ONE X4 | Yes               | Yes            |
| ONE RS | Yes               | Yes            |
| Ace Pro| Yes               | No             |
| Ace    | Yes               | No             |
| ONE X  | No                | Yes            |
| ONE X2 | No                | Yes            |
| ONE R  | No                | Yes            |

## Installation

```bash
go install github.com/xaionaro-go/insta360ctl/cmd/insta360ctl@latest
```

Or build from source:

```bash
git clone https://github.com/xaionaro-go/insta360ctl.git
cd insta360ctl
make build
```

## Usage

BLE operations require root privileges (or `CAP_NET_ADMIN`).

### Scan for cameras

```bash
sudo insta360ctl scan --timeout 10s
```

### Direct Control (Architecture B)

```bash
# Take a photo
sudo insta360ctl direct photo

# Start/stop recording
sudo insta360ctl direct record start
sudo insta360ctl direct record stop

# Change mode
sudo insta360ctl direct mode video
sudo insta360ctl direct mode photo
sudo insta360ctl direct mode timelapse

# Check battery and storage
sudo insta360ctl direct status
sudo insta360ctl direct battery
sudo insta360ctl direct storage

# Device info
sudo insta360ctl direct info

# Inject GPS coordinates
sudo insta360ctl direct gps 37.7749 -122.4194 10.0

# Set highlight marker
sudo insta360ctl direct marker

# Power off
sudo insta360ctl direct power-off

# Filter by address
sudo insta360ctl direct --addr AA:BB:CC:DD:EE:FF shutter
```

### GPS Remote (Architecture A)

```bash
# Start remote server (camera connects to us)
sudo insta360ctl remote start

# Send commands
sudo insta360ctl remote shutter
sudo insta360ctl remote mode
sudo insta360ctl remote screen
sudo insta360ctl remote power-off
```

### Video Preview Stream (WiFi)

First connect to the camera's WiFi AP (SSID like "GO 3 XXXX", default password: 88888888).

```bash
# Stream video to stdout, pipe to mpv for playback
insta360ctl stream | mpv --no-correct-pts --fps=30 -

# Save raw video to file
insta360ctl stream --output preview.h265

# Higher resolution
insta360ctl stream --resolution 1920x1080

# With duration limit
insta360ctl stream --duration 60s --output clip.h265
```

### WiFi Network Configuration

Instead of connecting to the camera's AP, you can tell the camera to join your
existing network. This is useful when you don't have a spare WiFi adapter.

```bash
# First connect to camera's AP, then tell it to join your LAN
insta360ctl wifi join --ssid "HomeWiFi" --password "secret"
# Output: Camera joined "HomeWiFi" with IP 192.168.1.42

# Now stream from the camera on your LAN
insta360ctl stream --addr 192.168.1.42

# Or do it all in one command
insta360ctl stream --wifi-ssid "HomeWiFi" --wifi-password "secret"

# Query camera's current WiFi connection
insta360ctl wifi info
```

## Architecture

The tool supports two BLE communication architectures and WiFi streaming:

- **Architecture A (GPS Remote)**: Emulates an Insta360 GPS Remote. The computer acts as a GATT server (CE80 service), and the camera connects as a client. Supports basic shutter/mode/power commands.

- **Architecture B (Direct Control)**: Connects to the camera's GATT server (BE80 service). Supports full camera control including capture modes, settings, GPS injection, and status queries.

- **WiFi Streaming**: Connects to the camera's WiFi TCP protocol (192.168.42.1:6666) for live video preview. Uses protobuf-encoded commands and multiplexed H.264/H.265 stream. Supports both AP mode (connect to camera's WiFi) and station mode (camera joins your network).

See [doc/protocol.md](doc/protocol.md) for protocol details.

## Requirements

- Linux with Bluetooth 4.0+ adapter (for BLE commands)
- WiFi connection to camera's AP (for streaming)
- Root privileges or `CAP_NET_ADMIN` capability (BLE only)
- Go 1.24+
