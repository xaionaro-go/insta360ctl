# Command Quick Reference

## Architecture A: GPS Remote (9-byte commands via CE82)

```
Shutter:   FC EF FE 86 00 03 01 02 00
Mode:      FC EF FE 86 00 03 01 01 00
Screen:    FC EF FE 86 00 03 01 00 00
Power Off: FC EF FE 86 00 03 01 00 03
```

## Architecture B: BLE Command Control (via BE81)

### Header16 Format (X3, ONE RS, Ace Pro)

Header template:
```
[len_lo] [len_hi] 00 00 04 00 00 [cmd_lo] [cmd_hi] 02 [seq] 00 00 80 00 00
```

### Go2BlePacket Format (GO 2, GO 3)

Full packet template:
```
FF 07 40 [size_lo] [size_hi] [16-byte inner header] [payload] [crc_lo] [crc_hi]
```

### Official Command Codes (from libOne.so MessageCode enum)

| Code | Hex  | Official Name | Payload | Notes |
|------|------|---------------|---------|-------|
| 0 | 0x00 | BEGIN | — | |
| 1 | 0x01 | START_LIVE_STREAM | Protobuf | Begin video preview |
| 2 | 0x02 | STOP_LIVE_STREAM | Protobuf | Stop video preview |
| 3 | 0x03 | TAKE_PICTURE | Protobuf (optional) | Verified: GO 3 |
| 4 | 0x04 | START_CAPTURE | Protobuf (optional) | Verified: GO 3 |
| 5 | 0x05 | STOP_CAPTURE | Protobuf (optional) | Verified: GO 3 |
| 6 | 0x06 | CANCEL_CAPTURE | — | |
| 7 | 0x07 | SET_OPTIONS | Protobuf | |
| 8 | 0x08 | GET_OPTIONS | Protobuf | |
| 9 | 0x09 | SET_PHOTOGRAPHY_OPTIONS | Protobuf | |
| 10 | 0x0A | GET_PHOTOGRAPHY_OPTIONS | Protobuf | |
| 11 | 0x0B | GET_FILE_EXTRA | Protobuf | |
| 12 | 0x0C | DELETE_FILES | Protobuf | GO 3 also uses for mode setting |
| 13 | 0x0D | GET_FILE_LIST | Protobuf | |
| 15 | 0x0F | GET_CURRENT_CAPTURE_STATUS | — | |
| 16 | 0x10 | SET_FILE_EXTRA | Protobuf | GO 3 returns 500 |
| 17 | 0x11 | GET_TIMELAPSE_OPTIONS | Protobuf | |
| 18 | 0x12 | SET_TIMELAPSE_OPTIONS | Protobuf | |
| 19 | 0x13 | GET_GYRO | — | |
| 22 | 0x16 | START_TIMELAPSE | Protobuf | |
| 23 | 0x17 | STOP_TIMELAPSE | Protobuf | |
| 24 | 0x18 | ERASE_SD_CARD | — | |
| 25 | 0x19 | CALIBRATE_GYRO | Protobuf | **GO 3: returns battery data** |
| 26 | 0x1A | SCAN_BT_PERIPHERAL | Protobuf | Not WiFi config! |
| 27 | 0x1B | CONNECT_TO_BT_PERIPHERAL | Protobuf | Not WiFi query! |
| 28 | 0x1C | DISCONNECT_BT_PERIPHERAL | Protobuf | |
| 29 | 0x1D | GET_CONNECTED_BT_PERIPHERALS | Protobuf | |
| 32 | 0x20 | REBOOT_CAMERA | — | |
| 33 | 0x21 | OPEN_CAMERA_WIFI | — | |
| 34 | 0x22 | CLOSE_CAMERA_WIFI | — | |
| 39 | 0x27 | CHECK_AUTHORIZATION | Protobuf | Verified: GO 3 |
| 40 | 0x28 | CANCEL_AUTHORIZATION | — | |
| 41 | 0x29 | START_BULLETTIME_CAPTURE | — | |
| 48 | 0x30 | STOP_BULLETTIME_CAPTURE | Protobuf | |
| 51 | 0x33 | START_HDR_CAPTURE | — | |
| 52 | 0x34 | STOP_HDR_CAPTURE | Protobuf | |
| 53 | 0x35 | UPLOAD_GPS | Protobuf | Verified: GO 3 |
| 54 | 0x36 | SET_SYNC_CAPTURE_MODE | Protobuf | |
| 55 | 0x37 | GET_SYNC_CAPTURE_MODE | — | |
| 56 | 0x38 | SET_STANDBY_MODE | Protobuf | |
| 57 | 0x39 | RESTORE_FACTORY_SETTINGS | — | |
| 60 | 0x3C | SET_KEY_TIME_POINT | Protobuf | Highlight marker. Verified: GO 3 |
| 86 | 0x56 | REQUEST_AUTHORIZATION | Protobuf | Verified: GO 3 |
| 87 | 0x57 | CANCEL_REQUEST_AUTHORIZATION | Protobuf | |

### WiFi Management Commands (higher code range)

| Code | Hex  | Official Name | Payload | Notes |
|------|------|---------------|---------|-------|
| 85 | 0x55 | SET_WIFI_SEIZE_ENABLE | Protobuf | WiFi exclusivity |
| 112 | 0x70 | SET_WIFI_CONNECTION_INFO | Protobuf | **Join WiFi network** |
| 113 | 0x71 | GET_WIFI_CONNECTION_INFO | — | Query WiFi status |
| 125 | 0x7D | RESET_WIFI | — | Reset WiFi config |
| 147 | 0x93 | SET_WIFI_MODE | Protobuf | Set AP/STA/P2P mode |
| 148 | 0x94 | GET_WIFI_SCAN_LIST | Protobuf | Trigger WiFi scan |
| 149 | 0x95 | GET_CONNECTED_WIFI_LIST | — | Saved WiFi networks |
| 150 | 0x96 | GET_WIFI_MODE | — | Query current mode |
| 175 | 0xAF | DEL_WIFI_HISTORY_INFO | Protobuf | Delete saved WiFi |

### Response Status Codes

| Code | HTTP Equivalent | Meaning |
|------|----------------|---------|
| 0x00C8 | 200 OK | Success |
| 0x0190 | 400 Bad Request | Unknown command |
| 0x01F4 | 500 Error | Execution error |
| 0x01F5 | 501 Not Implemented | Not supported |

### Camera Notifications (8192+ range)

| Code | Hex    | Official Name | Payload |
|------|--------|---------------|---------|
| 8193 | 0x2001 | FIRMWARE_UPGRADE_COMPLETE | — |
| 8195 | 0x2003 | BATTERY_UPDATE | `NotificationBatteryUpdate` |
| 8196 | 0x2004 | BATTERY_LOW | `NotificationBatteryLow` |
| 8197 | 0x2005 | SHUTDOWN | `NotificationShutdown` |
| 8198 | 0x2006 | STORAGE_UPDATE | `NotificationCardUpdate` |
| 8199 | 0x2007 | STORAGE_FULL | — |
| 8200 | 0x2008 | KEY_PRESSED | — |
| 8201 | 0x2009 | CAPTURE_STOPPED | — |
| 8208 | 0x2010 | CURRENT_CAPTURE_STATUS | `CaptureStatus` |
| 8209 | 0x2011 | AUTHORIZATION_RESULT | `NotificationAuthorizationResult` |
| 8214 | 0x2016 | CAM_TEMPERATURE_VALUE | — |
| 8217 | 0x2019 | CHARGE_BOX_BATTERY_UPDATE | — |
| 8232 | 0x2028 | WIFI_STATUS | `CameraWifiConnectionResult` |
| 8247 | 0x2037 | WIFI_MODE_CHANGE | `WifiModeResult` |
| 8249 | 0x2039 | WIFI_SCAN_LIST_CHANGED | `WifiScanInfoList` |

### WiFi Mode Values

| Value | Mode | Description |
|-------|------|-------------|
| 0 | AP | Camera as access point (default) |
| 1 | STA | Camera joins existing network |
| 2 | P2P | WiFi Direct |

### GO 3 Firmware Deviations

The GO 3 (firmware v1.4.51) deviates from the official protocol in several ways:

- **Battery query**: Command 0x19 (officially CALIBRATE_GYRO) returns battery status on GO 3
- **Storage**: Command 0x10 (SET_FILE_EXTRA) returns 500 error. Storage data comes via push notifications (0x2010)
- **WiFi**: The old 0x1A/0x1B commands were actually BT peripheral commands, not WiFi. The correct WiFi commands are 0x70/0x71

## WiFi Protocol (TCP port 6666)

Uses the same command codes as BLE. Connection is over TCP after joining the camera's WiFi AP.

### Stream Types

| Byte | Type | Format |
|------|------|--------|
| 0x20 | Video | Raw H.264/H.265 NAL units |
| 0x30 | Gyro/IMU | 48 bytes per sample (6 × float64 LE) |
| 0x40 | Sync | 32-byte timestamp data |
