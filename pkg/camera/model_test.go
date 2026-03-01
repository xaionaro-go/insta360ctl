package camera

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIdentifyFromBLEName(t *testing.T) {
	tests := []struct {
		name     string
		bleName  string
		expected Model
	}{
		{"X3 camera", "X3 ABCDEF", ModelX3},
		{"X4 camera", "X4 123456", ModelX4},
		{"X5 camera", "X5 AABBCC", ModelX5},
		{"ONE X3", "ONE X3 SN1234", ModelOneX3},
		{"ONE X2", "ONE X2 SN5678", ModelOneX2},
		{"ONE X4", "ONE X4 SN0000", ModelOneX4},
		{"ONE RS", "ONE RS SN1111", ModelOneRS},
		{"ONE R", "ONE R SN2222", ModelOneR},
		{"GO 3", "GO 3 AABB", ModelGo3},
		{"GO 3S", "GO 3S CCDD", ModelGo3S},
		{"Ace Pro", "Ace Pro 1234", ModelAcePro},
		{"Ace", "Ace 5678", ModelAce},
		{"unknown device", "SomeOtherDevice", ModelUnknown},
		{"empty name", "", ModelUnknown},
		{"partial match", "X", ModelUnknown},
		{"ONE fallback", "ONE XXXX", ModelOneX},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IdentifyFromBLEName(tt.bleName)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestIsInsta360Device(t *testing.T) {
	assert.True(t, IsInsta360Device("X3 ABC123"))
	assert.True(t, IsInsta360Device("X4 DEF456"))
	assert.False(t, IsInsta360Device("iPhone"))
	assert.False(t, IsInsta360Device(""))
}

func TestModelString(t *testing.T) {
	tests := []struct {
		model    Model
		expected string
	}{
		{ModelOneX, "ONE X"},
		{ModelOneX2, "ONE X2"},
		{ModelOneX3, "ONE X3"},
		{ModelOneX4, "ONE X4"},
		{ModelX3, "X3"},
		{ModelX4, "X4"},
		{ModelX5, "X5"},
		{ModelOneRS, "ONE RS"},
		{ModelOneR, "ONE R"},
		{ModelGo3, "GO 3"},
		{ModelGo3S, "GO 3S"},
		{ModelAcePro, "Ace Pro"},
		{ModelAce, "Ace"},
		{ModelUnknown, "Unknown"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.model.String())
	}
}

func TestSupportsDirectControl(t *testing.T) {
	assert.True(t, ModelX3.SupportsDirectControl())
	assert.True(t, ModelX4.SupportsDirectControl())
	assert.True(t, ModelX5.SupportsDirectControl())
	assert.True(t, ModelAcePro.SupportsDirectControl())
	assert.False(t, ModelOneX.SupportsDirectControl())
	assert.False(t, ModelUnknown.SupportsDirectControl())
}

func TestSupportsRemoteControl(t *testing.T) {
	assert.True(t, ModelOneX.SupportsRemoteControl())
	assert.True(t, ModelX3.SupportsRemoteControl())
	assert.True(t, ModelX4.SupportsRemoteControl())
	assert.False(t, ModelGo3.SupportsRemoteControl())
	assert.False(t, ModelAcePro.SupportsRemoteControl())
	assert.False(t, ModelUnknown.SupportsRemoteControl())
}

func TestParseCaptureMode(t *testing.T) {
	tests := []struct {
		input string
		mode  CaptureMode
		ok    bool
	}{
		{"photo", CaptureModePhoto, true},
		{"video", CaptureModeVideo, true},
		{"timelapse", CaptureModeTimelapse, true},
		{"hdr", CaptureModeHDR, true},
		{"bullettime", CaptureModeBulletTime, true},
		{"invalid", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		mode, ok := ParseCaptureMode(tt.input)
		assert.Equal(t, tt.ok, ok, "input: %s", tt.input)
		if ok {
			assert.Equal(t, tt.mode, mode)
		}
	}
}

func TestCaptureModeString(t *testing.T) {
	assert.Equal(t, "photo", CaptureModePhoto.String())
	assert.Equal(t, "video", CaptureModeVideo.String())
	assert.Equal(t, "timelapse", CaptureModeTimelapse.String())
	assert.Equal(t, "hdr", CaptureModeHDR.String())
	assert.Equal(t, "bullettime", CaptureModeBulletTime.String())
	assert.Equal(t, "unknown", CaptureMode(99).String())
}
