# Insta360 Protocol Specification

## Overview

Insta360 cameras support two BLE communication architectures and a WiFi protocol for streaming:

- **Architecture A (GPS Remote)**: Camera acts as GATT client, connecting to a remote's GATT server
- **Architecture B (Direct Control)**: Camera acts as GATT server, accepting connections from apps/controllers

## Architecture A: GPS Remote Emulation

### Service: CE80

| UUID | Direction | Properties | Description |
|------|-----------|------------|-------------|
| CE81 | Camera → Remote | Write | Camera writes data to remote |
| CE82 | Remote → Camera | Notify | Remote sends commands to camera |
| CE83 | Camera reads | Read | Static value: `0x02 0x01` |

### Secondary Service: D0FF

| UUID | Properties | Value |
|------|------------|-------|
| FFD1 | Read | Device name string |
| FFD2 | Read | Firmware version string |
| FFD3 | Read | `0x301e9001` |
| FFD4 | Read | `0x18002001` |

### Command Format (9 bytes)

```
FC EF FE 86 00 03 01 <action> <param>
```

| Command | Bytes | Description |
|---------|-------|-------------|
| Shutter | `FC EF FE 86 00 03 01 02 00` | Take photo or start/stop recording |
| Mode | `FC EF FE 86 00 03 01 01 00` | Cycle capture mode |
| Screen | `FC EF FE 86 00 03 01 00 00` | Wake display |
| Power Off | `FC EF FE 86 00 03 01 00 03` | Power off camera |

### Wake-on-BLE

The remote can wake a sleeping camera using iBeacon advertising:

```
Company ID: 0x004C (Apple)
Type: 0x02 (iBeacon)
UUID: "ORBIT" + 0x00 + <6-byte camera ID> + padding
Major: 0x0001
Minor: 0x0001
TX Power: -59 dBm (0xC5)
```

## Architecture B: BLE Command Control

### Service: BE80

| UUID | Direction | Properties | Description |
|------|-----------|------------|-------------|
| BE81 | App → Camera | Write | Send commands to camera |
| BE82 | Camera → App | Notify | Receive responses/events from camera |

### Wire Formats

Two wire formats exist for Architecture B, depending on the camera model:

#### Header16 Format (X3, ONE RS, Ace Pro, etc.)

Messages consist of a 16-byte header followed by an optional payload, sent directly
on BLE without additional framing:

```
Offset  Size  Description
0       2     Payload length (little-endian, excludes the 16-byte header)
2       2     Reserved (0x00)
4       1     Mode (always 0x04 = Message)
5       2     Reserved (0x00)
7       2     Command code (uint16 LE)
9       1     Content type (0x02 = protobuf)
10      1     Sequence number (1-254)
11      2     Reserved (0x00)
13      1     Flags: bit 7 = is_last_fragment, bit 6 = direction (0=app→cam, 1=cam→app)
14      2     Reserved (0x00)
```

#### Go2BlePacket Format (GO 2, GO 3, GO 3S)

Messages are wrapped in a 5-byte BLE header with a 2-byte CRC-16/MODBUS trailer:

```
[FF] [07] [40] [size_lo] [size_hi]   ← 5-byte BLE header
[inner header (16 bytes)]             ← same format as Header16
[protobuf payload]                    ← variable length
[CRC_lo] [CRC_hi]                    ← CRC-16/MODBUS over all preceding bytes
```

The inner 16-byte header uses the same logical layout as Header16, but bytes 0-3
are interpreted as a uint32 total inner size (= 16 + payload_length) rather than
a uint16 payload length.

See [ble_protocol.md](ble_protocol.md) for the full Go2BlePacket wire format.

### Sequence Numbers

- Range: 1-254 (0 and 255 are reserved)
- Each command uses the next sequence number
- Responses echo the sequence number for correlation
- Wraps from 254 back to 1

### Official Command Codes (from libOne.so MessageCode enum)

The complete command code table is extracted from the official Insta360 protobuf definitions.
Key codes used in this project:

| Code | Hex  | Official Name | Payload | Notes |
|------|------|---------------|---------|-------|
| 3 | 0x03 | TAKE_PICTURE | Protobuf (optional) | Verified: GO 3 |
| 4 | 0x04 | START_CAPTURE | Protobuf (optional) | Verified: GO 3 |
| 5 | 0x05 | STOP_CAPTURE | Protobuf (optional) | Verified: GO 3 |
| 7 | 0x07 | SET_OPTIONS | Protobuf | |
| 8 | 0x08 | GET_OPTIONS | Protobuf | |
| 12 | 0x0C | DELETE_FILES | Protobuf | *Not* SetCaptureMode |
| 16 | 0x10 | SET_FILE_EXTRA | Protobuf | *Not* GetStorageInfo |
| 18 | 0x12 | SET_TIMELAPSE_OPTIONS | Protobuf | *Not* GetBatteryInfo |
| 22 | 0x16 | START_TIMELAPSE | Protobuf | |
| 23 | 0x17 | STOP_TIMELAPSE | Protobuf | |
| 25 | 0x19 | CALIBRATE_GYRO | Protobuf | **GO 3: returns battery** |
| 26 | 0x1A | SCAN_BT_PERIPHERAL | Protobuf | *Not* WiFi config |
| 27 | 0x1B | CONNECT_TO_BT_PERIPHERAL | Protobuf | *Not* WiFi query |
| 39 | 0x27 | CHECK_AUTHORIZATION | Protobuf | Verified: GO 3 |
| 51 | 0x33 | START_HDR_CAPTURE | — | |
| 53 | 0x35 | UPLOAD_GPS | Protobuf | Verified: GO 3 |
| 60 | 0x3C | SET_KEY_TIME_POINT | Protobuf | Highlight marker |
| 86 | 0x56 | REQUEST_AUTHORIZATION | Protobuf | |
| 112 | 0x70 | SET_WIFI_CONNECTION_INFO | Protobuf | Join WiFi |
| 113 | 0x71 | GET_WIFI_CONNECTION_INFO | — | Query WiFi |
| 125 | 0x7D | RESET_WIFI | — | |
| 147 | 0x93 | SET_WIFI_MODE | Protobuf | AP/STA/P2P |
| 148 | 0x94 | GET_WIFI_SCAN_LIST | Protobuf | Trigger scan |
| 150 | 0x96 | GET_WIFI_MODE | — | Query mode |

See [command_reference.md](command_reference.md) for the full table.

### GO 3 Firmware Deviations

The GO 3 (firmware v1.4.51) deviates from the official protocol:

| Official Code | GO 3 Behavior | Notes |
|---------------|--------------|-------|
| 0x19 CALIBRATE_GYRO | Returns battery data | Level % and charging state |
| 0x10 SET_FILE_EXTRA | Returns 500 error | Storage comes via 0x2010 push notifications |
| 0x0C DELETE_FILES | Used for mode setting | GO 3 accepts mode byte as payload |
| 0x1A SCAN_BT_PERIPHERAL | Returns 200 OK for any payload | No-op on GO 3 |
| 0x1B CONNECT_TO_BT_PERIPHERAL | Returns 500 error | Not implemented on GO 3 |

### Response Status Codes (GO 3)

GO 3 responses use HTTP-style status codes in the command code field:

| Code | Status | Meaning |
|------|--------|---------|
| 0x00C8 | 200 OK | Command succeeded; payload contains response data |
| 0x0190 | 400 Bad Request | Unknown command or invalid parameters |
| 0x01F4 | 500 Error | Command execution failed |
| 0x01F5 | 501 Not Implemented | Command not supported by this camera |

### Camera Notification Codes (official enum)

| Code | Hex    | Official Name | Payload |
|------|--------|---------------|---------|
| 8193 | 0x2001 | FIRMWARE_UPGRADE_COMPLETE | — |
| 8195 | 0x2003 | BATTERY_UPDATE | `NotificationBatteryUpdate` |
| 8196 | 0x2004 | BATTERY_LOW | `NotificationBatteryLow` |
| 8197 | 0x2005 | SHUTDOWN | — |
| 8198 | 0x2006 | STORAGE_UPDATE | `NotificationCardUpdate` |
| 8199 | 0x2007 | STORAGE_FULL | — |
| 8200 | 0x2008 | KEY_PRESSED | — |
| 8201 | 0x2009 | CAPTURE_STOPPED | — |
| 8208 | 0x2010 | CURRENT_CAPTURE_STATUS | `CaptureStatus` |
| 8209 | 0x2011 | AUTHORIZATION_RESULT | `NotificationAuthorizationResult` |
| 8232 | 0x2028 | WIFI_STATUS | `CameraWifiConnectionResult` |
| 8247 | 0x2037 | WIFI_MODE_CHANGE | `WifiModeResult` |
| 8249 | 0x2039 | WIFI_SCAN_LIST_CHANGED | `WifiScanInfoList` |

#### GO 3 Observed Notification Codes

These codes were observed in real GO 3 traffic. Some overlap numerically with
the official notification enum but carry different semantic data on GO 3:

| Code | Hex    | Observed Data |
|------|--------|---------------|
| 0x2010 (8208) | CURRENT_CAPTURE_STATUS | Contains storage fields (free/total/files) |
| 0x2021 (8225) | — | Battery state updates |
| 0x2025 (8229) | — | Power state changes |
| 0x2026 (8230) | — | Periodic status (combined battery + storage) |

## Camera Identification

Insta360 cameras advertise with name prefixes:

| Prefix | Model |
|--------|-------|
| `X3 ` | Insta360 X3 |
| `X4 ` | Insta360 X4 |
| `X5 ` | Insta360 X5 |
| `ONE X3 ` | Insta360 ONE X3 |
| `ONE X2 ` | Insta360 ONE X2 |
| `ONE RS ` | Insta360 ONE RS |
| `Ace Pro ` | Insta360 Ace Pro |
| `GO 3 ` | Insta360 GO 3 |

The suffix after the prefix is typically a portion of the camera's serial number.

## WiFi Protocol

Insta360 cameras create a WiFi access point (AP mode). A proprietary TCP protocol on
port 6666 multiplexes commands, responses, keep-alives, and stream data on a single
connection. The same protocol is used regardless of whether the camera is in AP mode
or has been configured to join an existing network (station mode).

### WiFi Modes

The camera supports two WiFi operating modes:

- **AP Mode** (default): Camera creates its own WiFi access point. Clients connect
  directly to the camera.
- **Station Mode**: Camera joins an existing WiFi network. Useful when the controller
  and camera need to share the same LAN.

To switch from AP mode to station mode, send a `SET_WIFI_CONNECTION_INFO` command
(code 112) with the target network's SSID and password. The camera will attempt to
join the network and send a notification (code 8232) with the result.

### Connection Parameters

| Parameter | Value |
|-----------|-------|
| Camera IP (AP mode) | `192.168.42.1` |
| Camera IP (station mode) | Assigned by DHCP; reported in notification 8232 |
| TCP port | `6666` |
| WiFi SSID (AP mode) | Camera model + serial suffix (e.g., `GO 3 AABB`) |
| Default AP password | `88888888` (some firmware versions use random passwords) |

### TCP Packet Framing

Every TCP message is prefixed with a 4-byte little-endian length field. The length
value includes the 4 prefix bytes themselves:

```
Byte layout:
[L0] [L1] [L2] [L3] [P0] [P1] ... [Pn]
 └── uint32 LE ──┘   └── payload ──────┘

total_length = 4 + len(payload)
```

For example, a payload of `06 00 00 73 79 4E 63 65 4E 64 69 6E 53` (13 bytes)
is transmitted as:

```
11 00 00 00  06 00 00 73 79 4E 63 65 4E 64 69 6E 53
└─ 17 (LE) ─┘└────────── 13-byte payload ──────────┘
```

To read a packet:
1. Read 4 bytes → decode as uint32 LE → `total_length`
2. Read `total_length - 4` more bytes → that is the payload

### Packet Types

The first byte of each payload identifies the packet type. The second and third
bytes are always `0x00`:

| First Byte | Type Bytes | Name | Description |
|------------|------------|------|-------------|
| `0x01` | `01 00 00` | STREAM | Multiplexed stream data (video/gyro/sync) |
| `0x04` | `04 00 00` | MESSAGE | Protobuf command, response, or notification |
| `0x05` | `05 00 00` | KEEPALIVE | Connection keep-alive |
| `0x06` | `06 00 00` | SYNC | Synchronization handshake |

### Sync Handshake

Immediately after establishing the TCP connection, the client must perform a sync
handshake. The client sends a SYNC packet containing the ASCII magic string
`syNceNdinS`. The camera echoes back an identical SYNC packet.

```
Client → Camera:
  Payload: 06 00 00 73 79 4E 63 65 4E 64 69 6E 53
           └─type─┘ └─── "syNceNdinS" (ASCII) ───┘

Camera → Client:
  Payload: 06 00 00 73 79 4E 63 65 4E 64 69 6E 53  (identical echo)
```

If the camera does not echo the sync packet within 5 seconds, the connection
should be considered failed.

### Keep-Alive

After the sync handshake, the client should send a KEEPALIVE packet every 2 seconds
when no other data is being sent. The camera will disconnect if it receives no data
for approximately 10 seconds.

```
KEEPALIVE payload: 05 00 00
```

The camera may also send keep-alive packets to the client.

### Command/Response Format (MESSAGE Packets)

MESSAGE packets (`04 00 00`) carry protobuf-encoded commands, responses, and
notifications. They have a fixed 12-byte header followed by an optional protobuf body:

```
Offset  Size  Field           Description
──────  ────  ──────────────  ──────────────────────────────────────────
0       1     Packet type     Always 0x04
1-2     2     Reserved        Always 0x00 0x00
3-4     2     Message code    Command, response, or notification code (uint16 LE)
5       1     Content type    Always 0x02 (protobuf)
6-8     3     Sequence num    Request/response correlation (uint24 LE)
9       1     Flags           Always 0x80
10-11   2     Reserved        Always 0x00 0x00
12+     N     Body            Protobuf-encoded message (may be empty)
```

**Sequence numbers**: Each command sent by the client uses a monotonically increasing
sequence number (starting from 1). The camera echoes the sequence number in its
response, allowing the client to match responses to requests. Notifications from the
camera use sequence number 0.

### WiFi Command Codes

Commands are sent from the client (phone/app/controller) to the camera:

| Code | Name | Request Protobuf | Response |
|------|------|-----------------|----------|
| 0 | BEGIN | — | — |
| 1 | START_LIVE_STREAM | `StartLiveStream` | `StartLiveStreamResp` (empty) |
| 2 | STOP_LIVE_STREAM | `StopLiveStream` (empty) | `StopLiveStreamResp` (empty) |
| 3 | TAKE_PICTURE | `TakePicture` | — |
| 4 | START_CAPTURE | `StartCapture` | — |
| 5 | STOP_CAPTURE | `StopCapture` | — |
| 6 | CANCEL_CAPTURE | `CancelCapture` | — |
| 7 | SET_OPTIONS | `SetOptions` | — |
| 8 | GET_OPTIONS | `GetOptions` | `Options` |
| 13 | GET_FILE_LIST | `GetFileList` | file list |
| 15 | GET_CAPTURE_STATUS | — | `CameraCaptureStatus` |
| 33 | OPEN_CAMERA_WIFI | — | — |
| 34 | CLOSE_CAMERA_WIFI | — | — |
| 85 | SET_WIFI_SEIZE_ENABLE | `SetWifiSeizeEnable` | `SetWifiSeizeEnableResp` |
| 112 | SET_WIFI_CONNECTION_INFO | `SetWifiConnectionInfo` | `SetWifiConnectionInfoResp` |
| 113 | GET_WIFI_CONNECTION_INFO | `GetWifiConnectionInfo` | `GetWifiConnectionInfoResp` |

### Response Codes

Responses from the camera use the message code field to indicate status:

| Code | Meaning |
|------|---------|
| 200 | OK — command succeeded |
| 500 | Error — command execution failed |

### Camera Notifications

The camera sends unsolicited notifications using high message code values. These
arrive as MESSAGE packets with sequence number 0:

| Code | Name | Payload |
|------|------|---------|
| 8195 | BATTERY_UPDATE | Battery level data |
| 8196 | BATTERY_LOW | Low battery warning |
| 8197 | SHUTDOWN | Camera shutting down |
| 8198 | STORAGE_UPDATE | Storage state changed |
| 8199 | STORAGE_FULL | Storage full |
| 8201 | CAPTURE_STOPPED | Recording/capture stopped |
| 8208 | CURRENT_CAPTURE_STATUS | Current capture state |
| 8215 | CAM_WIFI_START | Camera WiFi started |
| 8232 | WIFI_CONNECTION_RESULT | WiFi join attempt result |
| 8249 | WIFI_SCAN_LIST_CHANGED | Available WiFi networks changed |

### Stream Data Format

Stream packets (type `01 00 00`) carry multiplexed real-time data. The stream
header is 12 bytes:

```
Offset  Size  Field           Description
──────  ────  ──────────────  ──────────────────────────────────────────
0       1     Packet type     Always 0x01
1-2     2     Reserved        Always 0x00 0x00
3       1     Stream type     Data channel identifier (see below)
4-11    8     Timestamp       Camera timestamp in microseconds (uint64 LE)
12+     N     Stream payload  Raw data (format depends on stream type)
```

**Stream type identifiers:**

| Byte | Type | Payload Format |
|------|------|---------------|
| `0x20` | Video | Raw H.264 or H.265 NAL units, directly pipeable to ffmpeg/ffplay |
| `0x30` | Gyro/IMU | Samples of 48 bytes each: 6 × float64 LE (ax, ay, az, gx, gy, gz) |
| `0x40` | Sync | 32-byte timestamp synchronization data |

### StartLiveStream Protobuf Message

The `StartLiveStream` message configures the video preview stream:

```protobuf
message StartLiveStream {
  bool            enable_audio       = 1;   // Enable audio data
  bool            enable_video       = 2;   // Enable video data (usually true)
  // field 3: audioType enum (AACBSType)
  uint32          audio_sample_rate  = 4;   // Audio sample rate in Hz
  uint32          audio_bitrate      = 5;   // Audio bitrate
  uint32          video_bitrate      = 6;   // Primary video bitrate (e.g., 40)
  VideoResolution resolution         = 7;   // Primary stream resolution
  bool            enable_gyro        = 8;   // Enable gyro/IMU data
  uint32          video_bitrate1     = 9;   // Secondary stream bitrate
  VideoResolution resolution1        = 10;  // Secondary stream resolution
  uint32          preview_stream_num = 11;  // Number of preview streams (0 or 1)
  bool            enable_rotate      = 12;  // Enable image rotation
  bool            is_for_live        = 13;  // True if for external RTMP streaming
}
```

**VideoResolution enum values:**

| Value | Resolution | Frame Rate |
|-------|-----------|------------|
| 0 | Unknown | — |
| 9 | 1440×720 | 30 fps |
| 18 | 480×240 | 30 fps |
| 29 | 1920×1080 | 30 fps |
| 34 | 424×240 | 15 fps |

### WiFi Network Configuration

The camera can be told to join an existing WiFi network using command 112
(`SET_WIFI_CONNECTION_INFO`). This is used by the official app to enable cloud
uploads and live streaming through an external network.

#### WifiConnectionInfo Message

```protobuf
message WifiConnectionInfo {
  string ssid     = 1;   // Network name (SSID)
  string bssid    = 2;   // Access point MAC address (optional)
  string password  = 3;   // Network password (WPA2)
  string ip_addr   = 4;   // IP address (populated in responses)
}
```

#### SetWifiConnectionInfo (Command 112)

Request the camera to join a WiFi network:

```protobuf
message SetWifiConnectionInfo {
  WifiConnectionInfo wifi_connection_info = 1;
}
```

The client sends this with `ssid` and `password` populated. The `bssid` field is
optional (can be empty to let the camera choose the best AP). The `ip_addr` field
is ignored in the request.

#### GetWifiConnectionInfo (Command 113)

Query the camera's current WiFi connection:

```protobuf
message GetWifiConnectionInfo {}   // empty request

message GetWifiConnectionInfoResp {
  WifiConnectionInfo wifi_connection_info = 1;
}
```

#### CameraWifiConnectionResult (Notification 8232)

After the camera attempts to join a network, it sends this notification:

```protobuf
enum WifiConnectionResult {
  SUCCESS              = 0;   // Connected successfully
  TIMEOUT              = 1;   // Connection attempt timed out
  ERROR_CONNECT_FAILED = 2;   // Connection failed (wrong password, etc.)
}

message CameraWifiConnectionResult {
  WifiConnectionResult wifi_connection_result = 1;
  WifiConnectionInfo   wifi_connection_info   = 2;  // ip_addr populated on success
}
```

#### WiFi Join Sequence

```
                          Camera (AP mode)
Client ──────────────────────────────────────── Camera
  │                                               │
  │  1. TCP connect to 192.168.42.1:6666          │
  │──────────────────────────────────────────────>│
  │                                               │
  │  2. Sync handshake (syNceNdinS)               │
  │<─────────────────────────────────────────────>│
  │                                               │
  │  3. SET_WIFI_CONNECTION_INFO (cmd 112)         │
  │     {ssid: "MyNet", password: "secret"}       │
  │──────────────────────────────────────────────>│
  │                                               │
  │  4. Response 200 OK (command accepted)         │
  │<──────────────────────────────────────────────│
  │                                               │
  │  5. WIFI_CONNECTION_RESULT (notif 8232)        │
  │     {result: SUCCESS, ip_addr: "192.168.1.42"}│
  │<──────────────────────────────────────────────│
  │                                               │
  │  6. Camera drops AP, joins "MyNet"             │
  │  ×× TCP connection lost ××                     │
  │                                               │

                          Camera (station mode)
Client ──────────────────────────────────────── Camera
  │                                               │
  │  7. TCP connect to 192.168.1.42:6666          │
  │──────────────────────────────────────────────>│
  │                                               │
  │  8. Sync + START_LIVE_STREAM (normal flow)     │
  │<─────────────────────────────────────────────>│
```

**Important notes:**

- After the camera joins the target network, it drops its WiFi AP. The original
  TCP connection will be lost.
- The notification (step 5) may arrive before or after the connection drops,
  depending on timing. If the connection drops after the OK response but before
  the notification, the join was likely successful.
- The camera's new IP address is reported in the notification's `ip_addr` field.
  If not reported, check your router's DHCP lease table.
- Some camera models may not support station mode or may require specific firmware
  versions.

#### SetWifiSeizeEnable (Command 85)

Controls whether the current WiFi client has exclusive access to the camera:

```protobuf
message SetWifiSeizeEnable {
  enum ConnectionState {
    unknown     = 0;
    monopolized = 1;   // Exclusive WiFi control
    seizable    = 2;   // WiFi can be taken by another client
  }
  ConnectionState state = 1;
}
```

### Complete Connection Flow

A typical session:

```
1. Join camera's WiFi AP (or have camera join your network)
2. TCP connect to camera_ip:6666
3. Send SYNC packet → receive SYNC echo
4. Start keep-alive timer (send every 2s when idle)
5. Send commands (START_LIVE_STREAM, SET_OPTIONS, etc.)
6. Receive responses (matched by sequence number) and notifications
7. Receive STREAM packets (video/gyro/sync) during live preview
8. Send STOP_LIVE_STREAM when done
9. Close TCP connection
```
