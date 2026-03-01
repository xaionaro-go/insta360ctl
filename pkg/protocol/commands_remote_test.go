package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoteCommandBytes(t *testing.T) {
	tests := []struct {
		name     string
		cmd      RemoteCommand
		expected []byte
	}{
		{
			name:     "shutter",
			cmd:      RemoteCommandShutter,
			expected: []byte{0xFC, 0xEF, 0xFE, 0x86, 0x00, 0x03, 0x01, 0x02, 0x00},
		},
		{
			name:     "mode",
			cmd:      RemoteCommandMode,
			expected: []byte{0xFC, 0xEF, 0xFE, 0x86, 0x00, 0x03, 0x01, 0x01, 0x00},
		},
		{
			name:     "screen",
			cmd:      RemoteCommandScreen,
			expected: []byte{0xFC, 0xEF, 0xFE, 0x86, 0x00, 0x03, 0x01, 0x00, 0x00},
		},
		{
			name:     "power off",
			cmd:      RemoteCommandPowerOff,
			expected: []byte{0xFC, 0xEF, 0xFE, 0x86, 0x00, 0x03, 0x01, 0x00, 0x03},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.cmd.Bytes())
			assert.Equal(t, 9, len(tt.cmd.Bytes()))
		})
	}
}

func TestRemoteCommandsSharePrefix(t *testing.T) {
	// All remote commands share the same 7-byte prefix.
	prefix := []byte{0xFC, 0xEF, 0xFE, 0x86, 0x00, 0x03, 0x01}

	cmds := []RemoteCommand{
		RemoteCommandShutter,
		RemoteCommandMode,
		RemoteCommandScreen,
		RemoteCommandPowerOff,
	}

	for _, cmd := range cmds {
		assert.Equal(t, prefix, cmd.Bytes()[:7])
	}
}
