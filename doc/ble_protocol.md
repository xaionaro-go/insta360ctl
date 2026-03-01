# Insta360 BLE Protocol Technical Specification

This document describes the BLE protocol used by Insta360 cameras for
Architecture B (BLE command control via BE80 service). It covers the
Go2BlePacket wire format used by GO 2/GO 3 cameras and the simpler Header16
format used by X3, ONE RS, and similar models.

---

## Table of Contents

1. [Go2BlePacket Format (5-byte BLE Header)](#1-go2blepacket-format-5-byte-ble-header)
2. [Inner Packet Format (16-byte Header)](#2-inner-packet-format-16-byte-header)
3. [CRC-16/MODBUS](#3-crc-16modbus)
4. [Connection and Sync Flow](#4-connection-and-sync-flow)
5. [Authorization Flow Details](#5-authorization-flow-details)
6. [Packet Reassembly](#6-packet-reassembly)
7. [Charging Box Sub-Protocol](#7-charging-box-sub-protocol)
8. [Native Protocol Implementation](#8-native-protocol-implementation)
9. [Complete Command List](#9-complete-command-list)
10. [Wire Format Summary](#10-wire-format-summary)
11. [GO 3-Specific Differences](#11-go-3-specific-differences)
12. [Key Implementation Notes](#12-key-implementation-notes)

---

## 1. Go2BlePacket Format (5-byte BLE Header)

The Go2BlePacket wraps inner data in a BLE-specific framing layer. All
variants share a 5-byte header starting with the magic byte `0xFF`. There are
three distinct variants depending on the packet's purpose.

### 1.1 Constants

| Constant | Value |
|---|---|
| BLE packet header size | **5** |
| Max compatible packet size | **500** |
| Max message content size | **486** (= 500 - 5 - 9) |

The max message content size is derived as:
`max_compatible_packet_size - ble_header_size - message_head_size` =
`500 - 5 - 9 = 486`.

### 1.2 Message Packet (Full Protocol Messages)

Used for all normal commands that carry protobuf-encoded payloads.

**Header format (5 bytes):**
```
Byte 0: 0xFF (magic marker, constant)
Byte 1: 0x07 (flags/constant)
Byte 2: 0x40 (packet type = Message)
Bytes 3-4: uint16 LE - inner data size (NOT including the 5-byte BLE header or the 2-byte CRC)
```

After the 5-byte header comes the inner Packet data (16-byte header + protobuf
payload). After the inner data comes a 2-byte CRC16/MODBUS (little-endian)
computed over ALL preceding bytes (BLE header + inner data).

**Total packet size** = 5 (BLE header) + inner_data_size + 2 (CRC) = inner_data_size + 7

**Encoding algorithm:**

The first 4 bytes are packed as a single uint32 LE:
```
bytes 0-3 = (inner_data_size << 24) | 0x004007FF
```

Breaking down `0x4007FF` as little-endian bytes:
- Byte 0: `0xFF` (magic marker)
- Byte 1: `0x07` (flags)
- Byte 2: `0x40` (packet type = Message)
- Byte 3: low byte of inner_data_size (placed by the shift `<< 24`)

Byte 4 is written separately as `inner_data_size >> 8` (the high byte of the
16-bit size).

### 1.3 Sync Packet (Sync/Handshake)

Used during the connection handshake to synchronize state with the camera.

**Header format (5 bytes):**
```
Byte 0: 0xFF (magic marker)
Byte 1: 0x07 (flags)
Byte 2: 0x41 (packet type = Sync) <-- differs from Message (0x40)
Bytes 3-4: uint16 LE - sync data size
```

The only difference from the Message packet is byte 2: `0x41` (Sync) instead of
`0x40` (Message). This tells the receiver to process the payload as a
synchronization packet rather than a command/protobuf message.

**Observed sync behavior (GO 3):**

The camera initiates sync by sending a packet containing a repeating `AB BA`
pattern (e.g., `AB BA AB BA AB BA AB BA AB BA`). The app must respond with a
sync packet containing 7 zero bytes (`00 00 00 00 00 00 00`).

### 1.4 Simplified FF-Frame Packet

Used for special operations like wake-up authorization. This variant has a
different header layout where bytes 1-2 carry semantic meaning (message type
and command code) rather than fixed constants.

**Header format (5 bytes):**
```
Byte 0: 0xFF (magic marker)
Byte 1: message type (e.g., 0x0C for wake-up auth, 0x06 for commands)
Byte 2: command code
Bytes 3-4: uint16 LE - data size (params only, NOT including header or CRC)
```

This format is used in contexts where the full inner Packet header (16 bytes) is
not needed, such as simple authorization handshakes.

### 1.5 Packet Size Detection

The receiver determines how many bytes to expect for a complete packet by
reading the uint16 LE value at bytes 3-4 of the incoming data. This requires
at least 4 bytes to be available. The full packet size is then computed as:
`inner_data_size + 7` (adding the 5-byte BLE header and 2-byte CRC).

If fewer than 4 bytes are available, the receiver signals "insufficient data"
and waits for more bytes.

---

## 2. Inner Packet Format (16-byte Header)

Inside the Go2BlePacket, the inner data uses a 16-byte header composed of two
parts: a 7-byte "packet head" and a 9-byte "message head."

### 2.1 Header Size Constants

| Constant | Value |
|---|---|
| Packet head size | **7** |
| Message head size | **9** |
| Stream head size | **9** |
| Tunnel head size | **14** |
| LinuxCmd head size | **3** |

The total inner header size for a Message packet is:
`packet_head_size + message_head_size` = `7 + 9` = **16 bytes**.

### 2.2 PacketType Enum

The PacketType determines how the packet body is interpreted:

| PacketType | Description |
|---|---|
| Message | Normal protobuf commands (the primary type) |
| Stream | Video/audio streaming data |
| Synchronize | Sync/handshake packets |
| SocketTunnel | Tunnel for TCP-like connections |
| LinuxCmd | Linux commands to the camera's OS |

The PacketType is read from byte 4 of the inner header.

### 2.3 Inner Packet Header Layout (16 bytes)

The 16-byte inner header is divided into two sections:

#### Packet Header (bytes 0-6, 7 bytes)

```
Bytes 0-3: uint32 LE = total_inner_size
                       (this is the ENTIRE inner data: 16-byte header + protobuf payload)
Byte 4:    0x04 = mode/PacketType (PacketType::Message)
Byte 5:    0x00 = flags
Byte 6:    0x00 = reserved
```

**Important:** Bytes 0-3 encode a **uint32** (not uint16!) representing the
total inner data size. The value includes the 16-byte header itself, so for a
protobuf payload of N bytes: `total_inner_size = 16 + N`.

For the Header16 format (X3, ONE RS), bytes 0-1 are interpreted as a uint16
payload length (excluding the header). For small payloads (< 65536 bytes), both
interpretations produce identical wire bytes because bytes 2-3 remain zero.

#### Message Header (bytes 7-15, 9 bytes)

The message header is packed as a uint64 at offset 7, with bit fields:

```
Bits  0-15:  MessageCode (uint16)           -> bytes 7-8
Bits 16-23:  MessageContentType (uint8)     -> byte 9 (0x02 for protobuf)
Bits 24-53:  fragment_offset (30 bits)      -> bytes 10-13 (lower 30 bits)
Bit  54:     MessageDirection               -> bit 6 of byte 13
Bit  55:     is_last_fragment               -> bit 7 of byte 13 (= 0x80 when set)
Bits 56-63:  continuation                   -> bytes 14-15
```

### 2.4 Complete Byte-Level Layout

For a **single-fragment protobuf command from phone to camera**, the 16-byte
inner header looks like:

```
Byte  0:  total_inner_size[0]  (uint32 LE, least significant byte)
Byte  1:  total_inner_size[1]
Byte  2:  total_inner_size[2]
Byte  3:  total_inner_size[3]  (uint32 LE, most significant byte)
Byte  4:  0x04                 (mode = PacketType::Message)
Byte  5:  0x00                 (flags)
Byte  6:  0x00                 (reserved)
Byte  7:  MessageCode[0]       (uint16 LE, low byte)
Byte  8:  MessageCode[1]       (uint16 LE, high byte)
Byte  9:  0x02                 (MessageContentType = protobuf)
Byte 10:  sequence_number      (low byte of fragment_offset, used as seq number)
Byte 11:  0x00                 (fragment_offset, continued)
Byte 12:  0x00                 (fragment_offset, continued)
Byte 13:  0x80                 (bit 7 = is_last_fragment = 1, bit 6 = direction = 0 = phone->camera)
Byte 14:  0x00                 (continuation)
Byte 15:  0x00                 (continuation)
```

### 2.5 MessageContentType Values

| Value | Meaning |
|---|---|
| 0x00 | Raw/binary |
| 0x02 | Protobuf-encoded |

### 2.6 MessageDirection Values

| Bit Value | Direction |
|---|---|
| 0 (bit 6 of byte 13 = 0) | Phone to Camera |
| 1 (bit 6 of byte 13 = 1) | Camera to Phone |

### 2.7 Fragment Flags (byte 13)

| Bit | Meaning |
|---|---|
| Bit 7 (0x80) | `is_last_fragment` -- set to 1 for single-fragment messages or the final fragment |
| Bit 6 (0x40) | `MessageDirection` -- 0 = phone->camera, 1 = camera->phone |
| Bits 0-5 | Upper bits of fragment_offset (bits 24-29 of the 30-bit offset) |

For a single-fragment phone-to-camera message: byte 13 = `0x80`.
For a single-fragment camera-to-phone response: byte 13 = `0xC0` (0x80 | 0x40).

### 2.8 Relationship to Header16 Format

The inner 16-byte header format matches the "Header16" format used by
X3/ONE RS cameras. The key differences:

1. **Bytes 0-3**: Header16 uses uint16 at offset 0 for payload length and
   treats bytes 2-3 as reserved zeros. The Go2 format uses a uint32 for
   `total_inner_size` (= 16 + payload_length). For small payloads, the wire
   bytes are identical.

2. **Bytes 7-8**: Header16 uses byte 7 as a single-byte command code. The Go2
   format uses a uint16 MessageCode spanning bytes 7-8. For command codes < 256,
   the wire bytes are identical.

---

## 3. CRC-16/MODBUS

### 3.1 Algorithm

**Function signature:** `uint16_t crc16Check(uint8_t *data, uint32_t length, uint16_t initial_value)`

**Parameters:**
- `data` -- Pointer to the data to checksum
- `length` -- Number of bytes to process
- `initial_value` -- Starting CRC value (always `0xFFFF` in practice)

**Algorithm (standard CRC-16/MODBUS):**
```
Polynomial: 0xA001 (bit-reversed form of the standard CRC-16 polynomial 0x8005)
Initial value: 0xFFFF
For each byte in data:
    CRC = CRC XOR byte
    Repeat 8 times:
        If (CRC AND 1) == 1:
            CRC = (CRC >> 1) XOR 0xA001
        Else:
            CRC = CRC >> 1
Output: 2 bytes, stored little-endian (low byte first, high byte second)
```

### 3.2 Reference Implementation

```c
uint16_t crc16Check(uint8_t *data, uint32_t length, uint16_t init) {
    uint16_t crc = init;  // always 0xFFFF
    for (uint32_t i = 0; i < length; i++) {
        crc ^= data[i];
        for (int bit = 0; bit < 8; bit++) {
            if (crc & 1) {
                crc = (crc >> 1) ^ 0xA001;
            } else {
                crc >>= 1;
            }
        }
    }
    return crc;
}
```

### 3.3 CRC Scope and Position

The CRC is computed over ALL bytes of the Go2BlePacket including:
- The 5-byte BLE header
- The complete inner data (16-byte inner header + protobuf payload)

The CRC does NOT include itself. It is stored as the last 2 bytes of the entire
Go2BlePacket, in little-endian order (low byte at position N-2, high byte at
position N-1).

**Position in a full protocol message:**
```
[5-byte BLE header] [inner data (N bytes)] [CRC low] [CRC high]
                                            ^-- CRC computed over everything before here
```

### 3.4 Verification Example

For a packet: `FF 07 40 10 00 <16 bytes inner header> <protobuf payload>`

The CRC is computed over: `FF 07 40 10 00` + all inner data bytes.

The result is appended as 2 bytes in little-endian order.

---

## 4. Connection and Sync Flow

### 4.1 BLE GATT Services

| Service UUID | Name | Purpose |
|---|---|---|
| `0xBE80` | BLE Command Control | Primary command service for all camera models |
| `0xB000` | Secondary | Response/notification service, used by GO 3 and similar models |
| `0xAE00` | Camera BLE | Camera BLE service, used by some models |
| `0x180A` | Device Info | Standard BLE Device Information Service |
| `0x180F` | Battery | Standard BLE Battery Service |
| Custom 128-bit | Action Pod | Model-specific identification (firmware version, capabilities) |

### 4.2 GATT Characteristics

| Characteristic UUID | Service | Properties | Purpose |
|---|---|---|---|
| `0xBE81` | `0xBE80` | Read, WriteNR, Write | Command writes (app to camera) |
| `0xBE82` | `0xBE80` | Notify | Command responses and camera events |
| `0xB001` | `0xB000` | Read, WriteNR, Write | Secondary command writes (GO 3 uses this) |
| `0xB002` | `0xB000` | Notify | Secondary notifications (channel 1) |
| `0xB003` | `0xB000` | Notify | Secondary notifications (channel 2) |
| `0xB004` | `0xB000` | Notify | Secondary notifications (channel 3) |
| `0xAE01` | `0xAE00` | Write | Camera BLE write characteristic |
| `0xAE02` | `0xAE00` | Notify/Read | Camera BLE notify/read characteristic |
| `0x2A19` | `0x180F` | Read, Notify | Standard Battery Level (0-100%) |

**Note:** Some camera models (e.g., GO 3) also expose a custom 128-bit UUID
service with additional characteristics for model-specific operations:

| Characteristic UUID Prefix | Properties | Purpose |
|---|---|---|
| `e3dd50bf...` | Write, Notify, Indicate | Command channel |
| `92e86c7a...` | Write | Write-only channel |
| `347f7608...` | Read | Read-only channel |

### 4.3 Connection Flow

The complete connection sequence:

**Step 1: Scan**
- Scan for BLE peripherals with known name prefixes: `"GO 3 "`, `"X3 "`,
  `"X4 "`, `"ONE "`, `"ONE X2 "`, `"ONE X3 "`, `"ONE RS "`, `"Ace Pro "`, etc.
- The suffix after the prefix is typically a portion of the camera's serial
  number.

**Step 2: Connect GATT**
- Establish GATT connection to the discovered peripheral.

**Step 3: Discover Services**
- Discover all available GATT services on the camera.

**Step 4: Discover Characteristics**
- For each service, discover all characteristics and their properties.

**Step 5: Set MTU**
- The official Insta360 app negotiates MTU to **247**, yielding ~244 bytes of
  usable payload per BLE write.
- BLE 5.x connections can use higher MTU values (e.g., 517).

**Step 6: Subscribe to Notifications**
- Subscribe (enable notifications) on the following characteristics:
  - `BE82` (primary responses)
  - `B002` (secondary channel 1)
  - `B003` (secondary channel 2)
  - `B004` (secondary channel 3)
- This is done by writing `0x0100` (little-endian for "enable notifications")
  to each characteristic's Client Characteristic Configuration Descriptor (CCCD).

**Step 7: Send Sync Packets**
- Create a sync packet and wrap it in a Go2BlePacket with type `0x41`
- Send the sync packet and wait for the camera's sync response
- On GO 3, the camera initiates sync after the first write: it sends a sync
  packet containing `AB BA AB BA...` pattern, and the app must respond with
  a sync packet containing 7 zero bytes

**Step 8: Handshake**
- The protocol includes a multi-step handshake between sync and authorization:
  - `REQ_SHAKE_HAND_STEP_1` — initial handshake request
  - `REQ_SHAKE_HAND_STEP_2` — handshake continuation
  - `CMD_SHAKE_HAND_RESP_ACK_OK` — camera acknowledges
- This may be the same logical operation as the sync packet exchange (step 7),
  tracked separately in the app's state machine

**Step 9: Authorize**
- Send `CHECK_AUTHORIZATION` (MessageCode = `0x27`)
- Include the authorization ID string in a `CheckAuthorization` protobuf message
- The camera responds with `AUTHORIZATION_RESULT` notification (0x2011)
- If the camera reports "not authorized":
  - Send `REQUEST_AUTHORIZATION` (MessageCode = `0x56`)
  - Camera responds with `AUTHORIZATION_RESULT` notification (0x2011)
  - The user must physically confirm on the camera (button press or screen tap)
- Error conditions:
  - Authorization busy (another device is pairing)
  - Authorization timeout
  - Authorization denied by user

**Step 10: Ready for Commands**
- Once authorization succeeds, the connection is fully established
- Normal protobuf commands can now be sent

### 4.4 Camera Identification by BLE Name

| Name Prefix | Camera Model |
|---|---|
| `GO 3 ` | Insta360 GO 3 |
| `GO 3S ` | Insta360 GO 3S |
| `X3 ` | Insta360 X3 |
| `X4 ` | Insta360 X4 |
| `X5 ` | Insta360 X5 |
| `ONE X3 ` | Insta360 ONE X3 |
| `ONE X2 ` | Insta360 ONE X2 |
| `ONE RS ` | Insta360 ONE RS |
| `ONE R ` | Insta360 ONE R |
| `Ace Pro ` | Insta360 Ace Pro |

---

## 5. Authorization Flow Details

### 5.1 Message Codes for Authorization

| Decimal | Hex | Name | Direction |
|---|---|---|---|
| 39 | `0x27` | `CHECK_AUTHORIZATION` | Phone -> Camera |
| 40 | `0x28` | `CANCEL_AUTHORIZATION` | Phone -> Camera |
| 86 | `0x56` | `REQUEST_AUTHORIZATION` | Phone -> Camera |
| 87 | `0x57` | `CANCEL_REQUEST_AUTHORIZATION` | Phone -> Camera |

Camera response codes are typically the same code + an offset, delivered as
notifications.

### 5.2 Protobuf Messages

#### CheckAuthorization (Phone -> Camera)

Protobuf package: `insta360.messages`
Message name: `CheckAuthorization`

| Field | Type | Description |
|---|---|---|
| `authorization_id` | `string` | Unique identifier for this app/device pairing |
| `findmy_token` | `string` (optional) | Token for Find My device integration |
| `initiator_type` | `CheckAuthorization_InitiatorType` (enum) | Who initiated the check |

#### CheckAuthorization_InitiatorType

| Value | Meaning |
|---|---|
| 0 | Default/Unknown |
| 1 | User-initiated |
| 2 | App-initiated |
| 3 | System-initiated |
| 16 | Background check |
| 32 | Automatic reconnection |

#### CheckAuthorizationResp (Camera -> Phone)

Message name: `CheckAuthorizationResp`

| Field | Type | Description |
|---|---|---|
| `authorization_status` | `CheckAuthorizationResp_AuthorizationStatus` (enum) | Current auth state |
| `findmy_pair_status` | `enum` (0-2) | Find My pairing status |

#### CheckAuthorizationResp_AuthorizationStatus

| Value | Meaning |
|---|---|
| 0 | Unknown/Uninitialized |
| 1 | Not Authorized |
| 2 | Authorization Pending (waiting for user confirmation) |
| 3 | Authorized |
| 4 | Authorization Expired |
| 5 | Authorization Denied |

#### RequestAuthorization (Phone -> Camera)

Message name: `RequestAuthorization`

| Field | Type | Description |
|---|---|---|
| `operation_type` | `AuthorizationOperationType` (enum) | What kind of authorization to request |

#### AuthorizationOperationType

| Value | Meaning |
|---|---|
| 0 | Default/standard authorization |
| 1 | Pair new device |
| 2 | Re-authorize existing device |
| 3 | Force re-pair |

#### CancelRequestAuthorization (Phone -> Camera)

Message name: `CancelRequestAuthorization`

Minimal fields -- cancels a pending authorization request.

#### NotificationAuthorizationResult (Camera -> Phone)

Message name: `NotificationAuthorizationResult`

Contains the result of a `RequestAuthorization` -- whether the user approved
or denied the pairing on the camera.

### 5.3 Authorization Sequence Diagram

```
Phone                                    Camera
  |                                        |
  |--- CheckAuthorization (0x27) --------->|
  |    [authorization_id, initiator_type]  |
  |                                        |
  |<-- CheckAuthorizationResp -------------|
  |    [authorization_status]              |
  |                                        |
  | (if status != Authorized)              |
  |                                        |
  |--- RequestAuthorization (0x56) ------->|
  |    [operation_type]                    |
  |                                        |
  |    (user presses button on camera)     |
  |                                        |
  |<-- NotificationAuthorizationResult ----|
  |    [success/denied]                    |
  |                                        |
  | (connection now authorized)            |
```

### 5.4 Wake-Up Authorization (Special Path)

The wake-up authorization uses the simplified FF-frame packet format instead of
the full protobuf protocol. This is used to authorize a connection when the
camera is in a low-power sleep state.

The wake-up authorization packet:
- `byte[0]` = `0xFF` (magic marker)
- `byte[1]` = `0x0C` (message type = wake-up)
- `byte[2]` = `0x02` (command = auth)
- `bytes[3-4]` = uint16 LE length of auth data
- `bytes[5+]` = authorization ID string bytes
- Last 2 bytes = CRC16/MODBUS

The wake-up authorization bypasses the normal sync + protobuf authorization
flow because the camera's full protocol stack may not be running in sleep mode.

---

## 6. Packet Reassembly

### 6.1 Receive Path

The main entry point for processing received BLE data strips the BLE framing
and reconstructs the inner Packet:

```
1. Read inner_data_size from bytes 3-4 (uint16 LE)
2. Verify: inner_data_size == total_received - 5 (header) - 2 (CRC)
3. Compute CRC16/MODBUS over all bytes except the last 2
4. Compare computed CRC with the 2-byte CRC at the end
5. Strip 5-byte BLE header and 2-byte CRC
6. Parse inner packet (16-byte header + protobuf payload)
7. Dispatch to handler based on MessageCode
```

### 6.2 Fragment Reassembly

For messages larger than the BLE MTU, the inner Packet header's fragment fields
are used:

- **fragment_offset** (bits 24-53 of the message header): Byte offset of this
  fragment within the complete message payload
- **is_last_fragment** (bit 55): Set to 1 for the last fragment

The receiver collects fragments until it sees `is_last_fragment = 1`, then
concatenates them in order of their fragment offsets to reconstruct the complete
protobuf payload.

For single-fragment messages (the common case for BLE commands), fragment_offset
is used as a sequence number and `is_last_fragment` is always set.

---

## 7. Charging Box Sub-Protocol

The Insta360 GO series cameras come with a charging case that has its own BLE
communication sub-protocol.

### 7.1 Constants

| Constant | Value |
|---|---|
| Charging head size | **2** |
| Box head size | **7** |

### 7.2 Structure

The charging box protocol uses the same CRC-16/MODBUS algorithm as the main
camera protocol.

### 7.3 Charging Box Notifications

The camera sends charging box status via these notification types:
- `CHARGE_BOX_BATTERY_UPDATE` (0x2019) — Battery level of the charging case
- `CHARGE_BOX_CONNECT_STATUS` (0x201E) — Whether the camera is seated in the
  charging case

---

## 8. Native Protocol Implementation

### 8.1 Call Flow for Sending a Command

```
App: sendCommand(cmdCode, protobufPayload)
  -> Create inner packet: 16-byte header + protobuf payload
  -> Wrap in Go2BlePacket: 5-byte BLE header + inner data + 2-byte CRC
  -> Write to BLE characteristic (BE81)
```

### 8.2 Call Flow for Receiving Data

```
BLE notification received on BE82/B002-B004:
  -> Read inner_data_size from bytes 3-4
  -> Verify CRC
  -> Strip BLE header and CRC
  -> Parse 16-byte inner header
  -> Extract protobuf payload
  -> Dispatch to handler based on MessageCode
```

### 8.3 Key Functions

| Function | Purpose |
|---|---|
| `newBlePacket` | Creates Message packet (type 0x40) |
| `newBleSyncPacket` | Creates Sync packet (type 0x41) |
| `newBlePacket2` | Creates simplified FF-frame packet |
| `probePacketSize` | Reads inner_data_size from bytes 3-4 |
| `crc16Check` | CRC-16/MODBUS computation |
| `parseBle` | Main BLE data parser |
| `sendWakeUpAuthorization` | Sends wake-up auth via simplified FF-frame |
| `sendHeartBeat` | Sends periodic heartbeat to keep connection alive |
| `writeBleSync` | Write sync packets during connection handshake |

---

## 9. Complete Command List

### 9.1 Phone Commands (Official MessageCode Enum)

These are commands sent from the phone/app to the camera. The MessageCode is
placed in bytes 7-8 of the inner packet header as a uint16 LE value.

The official names come from the `MessageCode` protobuf enum in `libOne.so`
(Insta360 Android SDK).

#### Core Commands (0x00-0x44)

| Code | Hex | Official Name | Payload | Notes |
|------|------|---------------|---------|-------|
| 0 | `0x00` | `BEGIN` | — | |
| 1 | `0x01` | `START_LIVE_STREAM` | Protobuf | Begin video preview |
| 2 | `0x02` | `STOP_LIVE_STREAM` | Protobuf | Stop video preview |
| 3 | `0x03` | `TAKE_PICTURE` | Protobuf (optional) | Verified: GO 3 |
| 4 | `0x04` | `START_CAPTURE` | Protobuf (optional) | Verified: GO 3 |
| 5 | `0x05` | `STOP_CAPTURE` | Protobuf (optional) | Verified: GO 3 |
| 6 | `0x06` | `CANCEL_CAPTURE` | — | |
| 7 | `0x07` | `SET_OPTIONS` | Protobuf | |
| 8 | `0x08` | `GET_OPTIONS` | Protobuf | |
| 9 | `0x09` | `SET_PHOTOGRAPHY_OPTIONS` | Protobuf | |
| 10 | `0x0A` | `GET_PHOTOGRAPHY_OPTIONS` | Protobuf | |
| 11 | `0x0B` | `GET_FILE_EXTRA` | Protobuf | |
| 12 | `0x0C` | `DELETE_FILES` | Protobuf | GO 3 also uses for mode setting |
| 13 | `0x0D` | `GET_FILE_LIST` | Protobuf | |
| 14 | `0x0E` | `TAKE_PICTURE_WITHOUT_STORING` | — | |
| 15 | `0x0F` | `GET_CURRENT_CAPTURE_STATUS` | — | |
| 16 | `0x10` | `SET_FILE_EXTRA` | Protobuf | GO 3 returns 500 |
| 17 | `0x11` | `GET_TIMELAPSE_OPTIONS` | Protobuf | |
| 18 | `0x12` | `SET_TIMELAPSE_OPTIONS` | Protobuf | |
| 19 | `0x13` | `GET_GYRO` | — | |
| 22 | `0x16` | `START_TIMELAPSE` | Protobuf | Verified: GO 3 |
| 23 | `0x17` | `STOP_TIMELAPSE` | Protobuf | Verified: GO 3 |
| 24 | `0x18` | `ERASE_SD_CARD` | — | |
| 25 | `0x19` | `CALIBRATE_GYRO` | Protobuf | **GO 3: returns battery data** |
| 26 | `0x1A` | `SCAN_BT_PERIPHERAL` | Protobuf | Not WiFi config! |
| 27 | `0x1B` | `CONNECT_TO_BT_PERIPHERAL` | Protobuf | Not WiFi query! |
| 28 | `0x1C` | `DISCONNECT_BT_PERIPHERAL` | Protobuf | |
| 29 | `0x1D` | `GET_CONNECTED_BT_PERIPHERALS` | Protobuf | |
| 30 | `0x1E` | `GET_MINI_THUMBNAIL` | — | |
| 31 | `0x1F` | `TEST_SD_CARD_SPEED` | — | |
| 32 | `0x20` | `REBOOT_CAMERA` | — | |
| 33 | `0x21` | `OPEN_CAMERA_WIFI` | — | |
| 34 | `0x22` | `CLOSE_CAMERA_WIFI` | — | |
| 35 | `0x23` | `OPEN_IPERF` | — | |
| 36 | `0x24` | `CLOSE_IPERF` | — | |
| 37 | `0x25` | `GET_IPERF_AVERAGE` | — | |
| 38 | `0x26` | `GET_FILE_INFO_LIST` | — | |
| 39 | `0x27` | `CHECK_AUTHORIZATION` | Protobuf | Verified: GO 3 |
| 40 | `0x28` | `CANCEL_AUTHORIZATION` | — | |
| 41 | `0x29` | `START_BULLETTIME_CAPTURE` | — | |
| 42 | `0x2A` | `SET_SUBMODE_OPTIONS` | Protobuf | |
| 43 | `0x2B` | `GET_SUBMODE_OPTIONS` | — | |
| 48 | `0x30` | `STOP_BULLETTIME_CAPTURE` | Protobuf | |
| 49 | `0x31` | `OPEN_OLED` | — | |
| 50 | `0x32` | `CLOSE_OLED` | — | |
| 51 | `0x33` | `START_HDR_CAPTURE` | — | |
| 52 | `0x34` | `STOP_HDR_CAPTURE` | Protobuf | |
| 53 | `0x35` | `UPLOAD_GPS` | Protobuf | Verified: GO 3 |
| 54 | `0x36` | `SET_SYNC_CAPTURE_MODE` | Protobuf | |
| 55 | `0x37` | `GET_SYNC_CAPTURE_MODE` | — | |
| 56 | `0x38` | `SET_STANDBY_MODE` | Protobuf | |
| 57 | `0x39` | `RESTORE_FACTORY_SETTINGS` | — | |
| 58 | `0x3A` | `SET_TEMP_OPTIONS_SWITCH` | Protobuf | |
| 59 | `0x3B` | `GET_TEMP_OPTIONS_SWITCH` | — | |
| 60 | `0x3C` | `SET_KEY_TIME_POINT` | Protobuf | Highlight marker. Verified: GO 3 |
| 61 | `0x3D` | `START_TIMESHIFT_CAPTURE` | — | |
| 62 | `0x3E` | `STOP_TIMESHIFT_CAPTURE` | Protobuf | |
| 63 | `0x3F` | `SET_FLOWSTATE_ENABLE` | Protobuf | |
| 64 | `0x40` | `GET_FLOWSTATE_ENABLE` | — | |
| 65 | `0x41` | `SET_ACTIVE_SENSOR` | Protobuf | |
| 66 | `0x42` | `GET_ACTIVE_SENSOR` | — | |
| 67 | `0x43` | `SET_MULTI_PHOTOGRAPHY_OPTIONS` | Protobuf | |
| 68 | `0x44` | `GET_MULTI_PHOTOGRAPHY_OPTIONS` | — | |

#### Extended Commands (0x47-0xC9)

| Code | Hex | Official Name | Payload | Notes |
|------|------|---------------|---------|-------|
| 71 | `0x47` | `GET_RECORDING_FILE` | — | |
| 83 | `0x53` | `PREPARE_GET_FILE_PACKAGE` | Protobuf | |
| 84 | `0x54` | `GET_FILE_PACKAGE_FINISH` | — | |
| 85 | `0x55` | `SET_WIFI_SEIZE_ENABLE` | Protobuf | WiFi exclusivity |
| 86 | `0x56` | `REQUEST_AUTHORIZATION` | Protobuf | Verified: GO 3 |
| 87 | `0x57` | `CANCEL_REQUEST_AUTHORIZATION` | Protobuf | |
| 103 | `0x67` | `SET_BUTTON_PRESS_PARAM` | Protobuf | |
| 104 | `0x68` | `GET_BUTTON_PRESS_PARAM` | — | |
| 105 | `0x69` | `I_FRAME_REQUEST` | — | |

#### WiFi Management Commands (0x70-0xAF)

| Code | Hex | Official Name | Payload | Notes |
|------|------|---------------|---------|-------|
| 112 | `0x70` | `SET_WIFI_CONNECTION_INFO` | Protobuf | **Join WiFi network** |
| 113 | `0x71` | `GET_WIFI_CONNECTION_INFO` | — | Query WiFi status |
| 118 | `0x76` | `SET_ACCESS_CAMERA_FILE_STATE` | Protobuf | |
| 120 | `0x78` | `SET_APP_ID` | Protobuf | |
| 125 | `0x7D` | `RESET_WIFI` | — | Reset WiFi config |
| 133 | `0x85` | `STOP_USB_CARD_BACKUP` | — | |
| 135 | `0x87` | `SET_CAMERA_LIVE_INFO` | Protobuf | |
| 136 | `0x88` | `GET_CAMERA_LIVE_INFO` | — | |
| 137 | `0x89` | `START_CAMERA_LIVE` | — | |
| 144 | `0x90` | `STOP_CAMERA_LIVE` | — | |
| 145 | `0x91` | `START_CAMERA_LIVE_RECORD` | — | |
| 146 | `0x92` | `STOP_CAMERA_LIVE_RECORD` | — | |
| 147 | `0x93` | `SET_WIFI_MODE` | Protobuf | Set AP/STA/P2P mode |
| 148 | `0x94` | `GET_WIFI_SCAN_LIST` | Protobuf | Trigger WiFi scan |
| 149 | `0x95` | `GET_CONNECTED_WIFI_LIST` | — | Saved WiFi networks |
| 150 | `0x96` | `GET_WIFI_MODE` | — | Query current mode |
| 151 | `0x97` | `PREPARE_GET_FILE_SYNC_PACKAGE` | Protobuf | |
| 152 | `0x98` | `GET_FILE_PACKAGE_SYNC_FINISH` | — | |
| 157 | `0x9D` | `DARK_EIS_STATUS` | Protobuf | |
| 160 | `0xA0` | `GET_CLOUD_STORAGE_UPLOAD_STATUS` | — | |
| 161 | `0xA1` | `SET_CLOUD_STORAGE_UPLOAD_STATUS` | Protobuf | |
| 162 | `0xA2` | `GET_CLOUD_STORAGE_BIND_STATUS` | — | |
| 163 | `0xA3` | `SET_CLOUD_STORAGE_BIND_STATUS` | Protobuf | |
| 164 | `0xA4` | `PAUSE_RECORDING` | — | |
| 167 | `0xA7` | `NOTIFY_OTA_ERROR` | Protobuf | |
| 172 | `0xAC` | `GET_DOWNLOAD_FILE_LIST` | Protobuf | |
| 173 | `0xAD` | `DOWNLOAD_INFO` | Protobuf | |
| 175 | `0xAF` | `DEL_WIFI_HISTORY_INFO` | Protobuf | Delete saved WiFi |
| 176 | `0xB0` | `SET_FAVORITE` | Protobuf | |
| 182 | `0xB6` | `QUICKREADER_GET_STATUS` | — | |
| 190 | `0xBE` | `ADD_DOWNLOAD_LIST_RESULT_SYNC` | Protobuf | |
| 201 | `0xC9` | `GET_EDIT_INFO_LIST` | Protobuf | |

### 9.2 Camera Notifications (Official Enum, 0x2000+ Range)

These are notifications sent from the camera to the phone. They arrive on the
notify characteristics (BE82, B002-B004). The MessageCode in notifications
uses codes in the 8192+ (0x2000+) range.

| Code | Hex | Official Name | Payload |
|------|--------|---------------|---------|
| 8193 | `0x2001` | `FIRMWARE_UPGRADE_COMPLETE` | — |
| 8194 | `0x2002` | `CAPTURE_AUTO_SPLIT` | Protobuf |
| 8195 | `0x2003` | `BATTERY_UPDATE` | `NotificationBatteryUpdate` |
| 8196 | `0x2004` | `BATTERY_LOW` | `NotificationBatteryLow` |
| 8197 | `0x2005` | `SHUTDOWN` | `NotificationShutdown` |
| 8198 | `0x2006` | `STORAGE_UPDATE` | `NotificationCardUpdate` |
| 8199 | `0x2007` | `STORAGE_FULL` | — |
| 8200 | `0x2008` | `KEY_PRESSED` | — |
| 8201 | `0x2009` | `CAPTURE_STOPPED` | — |
| 8202 | `0x200A` | `TAKE_PICTURE_STATE_UPDATE` | Protobuf |
| 8203 | `0x200B` | `DELETE_FILES_PROGRESS` | Protobuf |
| 8204 | `0x200C` | `PHONE_INSERT` | Protobuf |
| 8205 | `0x200D` | `BT_DISCOVER_PERIPHERAL` | Protobuf |
| 8206 | `0x200E` | `BT_CONNECTED_TO_PERIPHERAL` | Protobuf |
| 8207 | `0x200F` | `BT_DISCONNECTED_PERIPHERAL` | Protobuf |
| 8208 | `0x2010` | `CURRENT_CAPTURE_STATUS` | `CaptureStatus` |
| 8209 | `0x2011` | `AUTHORIZATION_RESULT` | `NotificationAuthorizationResult` |
| 8210 | `0x2012` | `TIMELAPSE_STATUS_UPDATE` | Protobuf |
| 8211 | `0x2013` | `SYNC_CAPTURE_MODE_UPDATE` | Protobuf |
| 8212 | `0x2014` | `SYNC_CAPTURE_BUTTON_TRIGGER` | Protobuf |
| 8213 | `0x2015` | `BT_REMOTE_VER_UPDATED` | Protobuf |
| 8214 | `0x2016` | `CAM_TEMPERATURE_VALUE` | — |
| 8215 | `0x2017` | `CAM_WIFI_START` | — |
| 8216 | `0x2018` | `CAM_BT_MSG_ANALYZE_FAILED` | — |
| 8217 | `0x2019` | `CHARGE_BOX_BATTERY_UPDATE` | — |
| 8219 | `0x201B` | `LIVEVIEW_BEGIN_ROTATE` | — |
| 8220 | `0x201C` | `EXPOSURE_UPDATE` | Protobuf |
| 8222 | `0x201E` | `CHARGE_BOX_CONNECT_STATUS` | Protobuf |
| 8232 | `0x2028` | `WIFI_STATUS` | `CameraWifiConnectionResult` |
| 8234 | `0x202A` | `UPDATE_LIVE_STREAM_PARAMS` | Protobuf |
| 8238 | `0x202E` | `FIRMWARE_UPGRADE_STATUS_TO_APP` | Protobuf |
| 8242 | `0x2032` | `USB_CARD_STATUS` | Protobuf |
| 8246 | `0x2036` | `CAMERA_LIVE_STATUS` | Protobuf |
| 8247 | `0x2037` | `WIFI_MODE_CHANGE` | `WifiModeResult` |
| 8248 | `0x2038` | `DATA_EXPORT_STATUS` | Protobuf |
| 8249 | `0x2039` | `WIFI_SCAN_LIST_CHANGED` | `WifiScanInfoList` |
| 8250 | `0x203A` | `DETECTED_FACE` | Protobuf |
| 8252 | `0x203C` | `DARK_EIS_STATUS` | Protobuf |
| 8255 | `0x203F` | `CLOUD_STORAGE_BIND_STATUS` | Protobuf |
| 8256 | `0x2040` | `SUPPORT_TAKE_PHOTO_ON_REC_STATUS` | Protobuf |
| 8259 | `0x2043` | `NEED_DOWNLOAD_FILE` | Protobuf |
| 8270 | `0x204E` | `INTERVAL_REC_INFO` | Protobuf |
| 8275 | `0x2053` | `CAM_SUBMODE_CHANGE` | Protobuf |
| 8279 | `0x2057` | `DELETE_FILE_RESULT` | Protobuf |
| 8284 | `0x205C` | `FAVORITE_CHANGE_STATUS` | Protobuf |
| 8285 | `0x205D` | `USER_TAKEOVER` | Protobuf |

#### GO 3 Observed Notification Mappings

On GO 3, the following notifications have been observed in real camera traffic.
The numeric codes match the official enum but the GO 3 firmware may use some
codes with slightly different payload semantics:

| Code | Hex | Official Name | GO 3 Usage |
|------|--------|---------------|------------|
| 8198 | `0x2006` | `STORAGE_UPDATE` | Capture state updates |
| 8202 | `0x200A` | `TAKE_PICTURE_STATE_UPDATE` | Device info updates |
| 8208 | `0x2010` | `CURRENT_CAPTURE_STATUS` | Storage state (see section 11) |
| 8225 | `0x2021` | *(not in official enum)* | Battery state |
| 8229 | `0x2025` | *(not in official enum)* | Power state |
| 8230 | `0x2026` | *(not in official enum)* | Periodic status |

### 9.3 Response Status Codes

Response packets from the camera use the MessageCode field with HTTP-style
status codes:

| Code | Hex | HTTP Equivalent | Meaning |
|------|--------|----------------|---------|
| 200 | `0x00C8` | 200 OK | Success |
| 400 | `0x0190` | 400 Bad Request | Unknown command or invalid parameters |
| 500 | `0x01F4` | 500 Error | Execution error |
| 501 | `0x01F5` | 501 Not Implemented | Not supported |

### 9.4 WiFi Mode Values

| Value | Mode | Description |
|-------|------|-------------|
| 0 | AP | Camera as access point (default) |
| 1 | STA | Camera joins existing network |
| 2 | P2P | WiFi Direct |

### 9.5 Capture Modes

| Value | Mode |
|---|---|
| `0x00` | Photo |
| `0x01` | Video |
| `0x02` | Timelapse |
| `0x03` | HDR Photo |
| `0x04` | Bullet Time |

---

## 10. Wire Format Summary

### 10.1 Full Protocol Message (for normal commands)

This is the primary packet format used for all protobuf-encoded commands and
responses between the phone and camera.

```
+-------+-------+-------+-------+-------+-------+-------+  ...  +-------+-------+
| BLE Header (5 bytes)          | Inner Packet Header (16 bytes) |       | CRC   |
+-------+-------+-------+-------+-------+-------+-------+  ...  +-------+-------+

Detailed layout:

[5-byte BLE Header]
  Byte 0:    0xFF                    (magic marker)
  Byte 1:    0x07                    (flags, constant)
  Byte 2:    0x40                    (packet type = Message)
  Bytes 3-4: inner_data_size         (uint16 LE = 16 + protobuf_payload_length)

[16-byte Inner Packet Header]
  Bytes 0-3:  total_inner_size       (uint32 LE = 16 + protobuf_payload_length)
  Byte 4:     0x04                   (mode = PacketType::Message)
  Byte 5:     0x00                   (flags)
  Byte 6:     0x00                   (reserved)
  Bytes 7-8:  MessageCode            (uint16 LE)
  Byte 9:     0x02                   (ContentType = protobuf)
  Byte 10:    sequence_number        (1-254, wraps around)
  Bytes 11-12: 0x0000               (reserved)
  Byte 13:    0x80                   (is_last_fragment=1, direction=0=phone->camera)
  Bytes 14-15: 0x0000               (reserved)

[Protobuf Payload]
  Variable length serialized protobuf message (may be 0 bytes for parameterless commands)

[2-byte CRC16]
  CRC16/MODBUS over ALL preceding bytes (BLE header + inner header + payload)
  Stored as: [low_byte] [high_byte]
```

**Total packet size:** 5 + 16 + protobuf_length + 2 = protobuf_length + 23

**Note on inner_data_size vs total_inner_size:** Both the BLE header's
inner_data_size (bytes 3-4) and the inner packet header's total_inner_size
(bytes 0-3) should have the same value: `16 + protobuf_payload_length`. The
BLE header stores it as uint16 LE, the inner header as uint32 LE.

### 10.2 Simplified FF-Frame (for wake-up auth, etc.)

This format is used for special operations where the full protobuf protocol is
not needed, such as wake-up authorization.

```
[5-byte BLE Header]
  Byte 0:    0xFF                    (magic marker)
  Byte 1:    message_type            (e.g., 0x0C for wake-up auth, 0x06 for commands)
  Byte 2:    command_code            (e.g., 0x02 for auth command)
  Bytes 3-4: data_size              (uint16 LE, length of data section only)

[Data Section]
  Variable length parameters/payload

[2-byte CRC16]
  CRC16/MODBUS over ALL preceding bytes (BLE header + data section)
  Stored as: [low_byte] [high_byte]
```

**Total packet size:** 5 + data_length + 2 = data_length + 7

### 10.3 Sync Packet

Used during the connection handshake (step 7 of the connection flow).

```
[5-byte BLE Header]
  Byte 0:    0xFF                    (magic marker)
  Byte 1:    0x07                    (flags, constant)
  Byte 2:    0x41                    (packet type = Sync)
  Bytes 3-4: sync_data_size         (uint16 LE)

[Sync Data]
  Content for synchronization
  (camera sends: AB BA AB BA... pattern; app responds: 7 zero bytes)

[2-byte CRC16]
  CRC16/MODBUS over ALL preceding bytes (BLE header + sync data)
  Stored as: [low_byte] [high_byte]
```

**Total packet size:** 5 + sync_data_size + 2 = sync_data_size + 7

### 10.4 Complete Packet Example

**Example: Sending a TakePhoto command (MessageCode 0x03, no protobuf payload)**

Inner data size: 16 (header only, no payload)

```
BLE Header:     FF 07 40 10 00
                ^  ^  ^  ^^^^^
                |  |  |  inner_data_size = 0x0010 = 16 (LE)
                |  |  packet type = Message
                |  flags
                magic

Inner Header:   10 00 00 00 04 00 00 03 00 02 01 00 00 80 00 00
                ^^^^^^^^^^^                ^^^^^       ^^
                total_inner_size=16        MsgCode=3   is_last_fragment
                (uint32 LE)                (TakePhoto)

CRC:            XX XX  (CRC16/MODBUS of all preceding 21 bytes)
```

Total packet: 5 + 16 + 2 = 23 bytes.

---

## 11. GO 3-Specific Differences

The GO 3 uses the Go2BlePacket wire format (with CRC framing) and has several
protocol differences from Header16 cameras (X3, ONE RS, Ace Pro):

### 11.1 Battery Query

- **Header16 cameras** may use command code `0x12` (`SET_TIMELAPSE_OPTIONS` in
  the official enum — the battery query semantic at this code is model-specific)
- **GO 3** uses command code `0x19` (officially `CALIBRATE_GYRO`, but the GO 3
  firmware repurposes it to return battery info)
- The GO 3 battery response is protobuf-encoded:
  - Field 1 (varint): battery level percentage (0-100)
  - Field 2 (varint): charging state (0=not charging, 1=charging)

### 11.2 Storage Query

- **Header16 cameras** may use command code `0x10` (officially `SET_FILE_EXTRA`,
  but some models repurpose it for storage query)
- **GO 3** returns HTTP 500 error for command `0x10`
- Instead, GO 3 pushes storage state via unsolicited `0x2010` notifications:
  - Field 1 (varint): capture state (0=idle, 1=post-recording, 4=recording)
  - Field 4 (varint): free capacity
  - Field 5 (varint): file count
  - Field 6 (varint): total capacity

### 11.3 Response Status Codes

GO 3 responses use HTTP-style status codes in the MessageCode field of the
response packet:

| Code | HTTP Equivalent | Meaning |
|------|----------------|---------|
| 0x00C8 | 200 OK | Command succeeded |
| 0x0190 | 400 Bad Request | Unknown command or invalid parameters |
| 0x01F4 | 500 Error | Command execution failed |
| 0x01F5 | 501 Not Implemented | Command not supported |

### 11.4 Write Characteristic Selection

Although GO 3 exposes multiple write characteristics (BE81, AE01, B001),
real-device testing shows that **BE81** is the correct write characteristic.
The camera only responds to commands sent on BE81.

### 11.5 Camera-Initiated Sync

After the first write to any characteristic, the GO 3 sends a sync packet
containing a repeating `AB BA` pattern (e.g., 10 bytes: `AB BA AB BA AB BA AB BA AB BA`).
The app must respond with a sync packet containing **7 zero bytes**. If the
sync response is not sent, subsequent commands may be silently dropped.

---

## 12. Key Implementation Notes

### 12.1 Sync Handshake is Required

The sync handshake (sending a sync packet and waiting for a sync response) is
**mandatory** before any commands will be processed. The camera checks whether
sync has been completed before handling messages. Without completing the sync,
the camera silently drops all incoming command packets.

### 12.2 Authorization is Required

The authorization check verifies that the connection is opened before proceeding
with any camera operations. The authorization flow uses protobuf messages
(`CheckAuthorization` / `RequestAuthorization`) and requires the camera to
acknowledge the pairing.

For cameras in sleep mode, the wake-up authorization path must be used instead,
which uses the simplified FF-frame format.

### 12.3 Inner Header Size Field is uint32, Not uint16

The inner 16-byte header format uses bytes 0-3 as a **uint32** (total inner
size), not a **uint16** (payload length) + 2 reserved bytes.

For payloads smaller than 65536 bytes, the uint16 interpretation produces
identical wire bytes because bytes 2-3 remain zero. However, for very large
payloads (such as firmware update data), the uint32 interpretation is required.

### 12.4 CRC Covers the Entire Packet

The CRC-16/MODBUS checksum is computed over the **entire** packet including the
5-byte BLE header, not just the inner data. This means:

```
CRC input = [5-byte BLE header] + [inner data (16-byte header + protobuf payload)]
CRC output = 2 bytes appended at the end
```

### 12.5 MTU Considerations

- The official app negotiates MTU to **247**, yielding ~244 bytes of usable
  payload per BLE write.
- BLE 5.x connections can use higher MTU values (e.g., 517).
- The max compatible packet size is **500**, meaning packets up to 500 bytes of
  inner data are supported.
- For packets larger than the MTU, they must be split across multiple BLE write
  operations. The receiver reassembles based on the size field in the BLE
  header.

### 12.6 Sequence Numbers

- Sequence numbers cycle from 1 to 254 (0 and 255 are reserved).
- Each command uses the next sequence number.
- Responses from the camera echo the sequence number for correlation.
- The sequence number is stored in byte 10 of the inner packet header (the
  low byte of the `fragment_offset` field).

---

## Appendix A: Architecture A (GPS Remote) Protocol

Architecture A is a completely separate protocol used for GPS remote emulation.
It uses a different service (CE80) and a fixed 9-byte command format. See
`doc/protocol.md` for full details on Architecture A.

The GPS remote protocol is not covered by the Go2BlePacket framing described
in this document. It uses its own fixed-format commands:

```
FC EF FE 86 00 03 01 <action> <param>
```

## Appendix B: Wake-on-BLE (iBeacon)

The GPS remote can wake a sleeping camera using iBeacon advertising:

```
Company ID: 0x004C (Apple)
Type: 0x02 (iBeacon)
UUID: "ORBIT" + 0x00 + <6-byte camera ID> + padding
Major: 0x0001
Minor: 0x0001
TX Power: -59 dBm (0xC5)
```

This is separate from the wake-up authorization BLE command described in
section 5.4, which operates over an established GATT connection.
