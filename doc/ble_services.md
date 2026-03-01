# BLE GATT Services

## Architecture A: GPS Remote

### Primary Service: CE80

UUID: `0000CE80-0000-1000-8000-00805F9B34FB`

| Characteristic | UUID | Properties | Description |
|---------------|------|------------|-------------|
| CE81 | `0000CE81-0000-1000-8000-00805F9B34FB` | Write | Camera → Remote data |
| CE82 | `0000CE82-0000-1000-8000-00805F9B34FB` | Notify | Remote → Camera commands |
| CE83 | `0000CE83-0000-1000-8000-00805F9B34FB` | Read | Static: `0x02 0x01` |

### Secondary Service: D0FF

UUID: `0000D0FF-3C17-D293-8E48-14FE2E4DA212`

| Characteristic | UUID | Properties | Value |
|---------------|------|------------|-------|
| FFD1 | `0000FFD1-3C17-D293-8E48-14FE2E4DA212` | Read | Device name |
| FFD2 | `0000FFD2-3C17-D293-8E48-14FE2E4DA212` | Read | Firmware version |
| FFD3 | `0000FFD3-3C17-D293-8E48-14FE2E4DA212` | Read | `0x301e9001` |
| FFD4 | `0000FFD4-3C17-D293-8E48-14FE2E4DA212` | Read | `0x18002001` |

## Architecture B: BLE Command Control

### Service: BE80

UUID: `0000BE80-0000-1000-8000-00805F9B34FB`

| Characteristic | UUID | Properties | Description |
|---------------|------|------------|-------------|
| BE81 | `0000BE81-0000-1000-8000-00805F9B34FB` | Write | App → Camera commands |
| BE82 | `0000BE82-0000-1000-8000-00805F9B34FB` | Notify | Camera → App responses |

### Secondary Service: B000 (GO 3 and similar)

UUID: `0000B000-0000-1000-8000-00805F9B34FB`

| Characteristic | UUID | Properties | Description |
|---------------|------|------------|-------------|
| B001 | `0000B001-0000-1000-8000-00805F9B34FB` | Read, Write, WriteNR | App → Camera commands (secondary for GO 3) |
| B002 | `0000B002-0000-1000-8000-00805F9B34FB` | Notify | Camera → App notifications (channel 1) |
| B003 | `0000B003-0000-1000-8000-00805F9B34FB` | Notify | Camera → App notifications (channel 2) |
| B004 | `0000B004-0000-1000-8000-00805F9B34FB` | Notify | Camera → App notifications (channel 3) |

### Camera BLE Service: AE00

UUID: `0000AE00-0000-1000-8000-00805F9B34FB`

Used by some camera models as an alternate communication service.

| Characteristic | UUID | Properties | Description |
|---------------|------|------------|-------------|
| AE01 | `0000AE01-0000-1000-8000-00805F9B34FB` | Write | App → Camera commands |
| AE02 | `0000AE02-0000-1000-8000-00805F9B34FB` | Notify, Read | Camera → App responses |

### Standard BLE Services

| Service | UUID | Description |
|---------|------|-------------|
| Device Info | `0x180A` | Standard Device Information Service |
| Battery | `0x180F` | Standard Battery Service (char: 0x2A19 = Battery Level) |
