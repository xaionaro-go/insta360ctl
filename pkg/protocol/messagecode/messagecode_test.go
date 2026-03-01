package messagecode

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodeString(t *testing.T) {
	tests := []struct {
		code     Code
		expected string
	}{
		{CodeTakePhoto, "TakePicture"},
		{CodeStartRecording, "StartCapture"},
		{CodeStopRecording, "StopCapture"},
		{CodeSetGPS, "UploadGPS"},
		{CodeSetHighlight, "SetKeyTimePoint"},
		{CodeNotifyBatteryUpdate, "Notify:BatteryUpdate"},
		{CodeResponseOK, "OK(200)"},
		{CodeResponseError, "Error(500)"},
		{Code(0xFE), "Unknown(0x00FE)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code.String())
		})
	}
}

func TestCodeValues(t *testing.T) {
	// Verify critical command codes match expected hex values from official MessageCode enum.
	assert.Equal(t, Code(0x03), CodeTakePicture)
	assert.Equal(t, Code(0x04), CodeStartCapture)
	assert.Equal(t, Code(0x05), CodeStopCapture)
	assert.Equal(t, Code(0x16), CodeStartTimelapse)
	assert.Equal(t, Code(0x17), CodeStopTimelapse)
	assert.Equal(t, Code(0x19), CodeCalibrateGyro)
	assert.Equal(t, Code(0x1A), CodeScanBTPeripheral)
	assert.Equal(t, Code(0x1B), CodeConnectToBTPeripheral)
	assert.Equal(t, Code(0x27), CodeCheckAuthorization)
	assert.Equal(t, Code(0x33), CodeStartHDRCapture)
	assert.Equal(t, Code(0x35), CodeUploadGPS)
	assert.Equal(t, Code(0x3C), CodeSetKeyTimePoint)
	assert.Equal(t, Code(0x56), CodeRequestAuthorization)
	assert.Equal(t, Code(0x70), CodeSetWifiConnectionInfo)
	assert.Equal(t, Code(0x71), CodeGetWifiConnectionInfo)
	assert.Equal(t, Code(0x93), CodeSetWifiMode)
	assert.Equal(t, Code(0x96), CodeGetWifiMode)
	assert.Equal(t, Code(0x94), CodeGetWifiScanList)
	assert.Equal(t, Code(0x95), CodeGetConnectedWifiList)
	assert.Equal(t, Code(0x7D), CodeResetWifi)

	// Backward-compatible aliases.
	assert.Equal(t, CodeTakePicture, CodeTakePhoto)
	assert.Equal(t, CodeStartCapture, CodeStartRecording)
	assert.Equal(t, CodeStopCapture, CodeStopRecording)
	assert.Equal(t, CodeStartHDRCapture, CodeSetHDR)
	assert.Equal(t, CodeStartTimelapse, CodeSetTimelapse)
	assert.Equal(t, CodeUploadGPS, CodeSetGPS)
	assert.Equal(t, CodeSetKeyTimePoint, CodeSetHighlight)
	assert.Equal(t, CodeCalibrateGyro, CodeGo3GetBattery)
}
