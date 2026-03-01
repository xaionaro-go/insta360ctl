package protocol

// RemoteCommand represents a 9-byte command sent from the GPS Remote
// to the camera via Architecture A (CE82 notifications).
type RemoteCommand [9]byte

// Pre-defined GPS Remote commands.
// Format: FC EF FE 86 00 03 01 <action> <param>
var (
	// RemoteCommandShutter triggers the shutter (take photo or start/stop recording).
	RemoteCommandShutter = RemoteCommand{0xFC, 0xEF, 0xFE, 0x86, 0x00, 0x03, 0x01, 0x02, 0x00}

	// RemoteCommandMode cycles through camera modes.
	RemoteCommandMode = RemoteCommand{0xFC, 0xEF, 0xFE, 0x86, 0x00, 0x03, 0x01, 0x01, 0x00}

	// RemoteCommandScreen turns the camera screen on.
	RemoteCommandScreen = RemoteCommand{0xFC, 0xEF, 0xFE, 0x86, 0x00, 0x03, 0x01, 0x00, 0x00}

	// RemoteCommandPowerOff powers off the camera.
	RemoteCommandPowerOff = RemoteCommand{0xFC, 0xEF, 0xFE, 0x86, 0x00, 0x03, 0x01, 0x00, 0x03}
)

// Bytes returns the command as a byte slice.
func (c RemoteCommand) Bytes() []byte {
	return c[:]
}
