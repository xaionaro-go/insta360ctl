package camera

import "context"

// Camera is a unified interface for controlling an Insta360 camera
// regardless of the underlying BLE architecture (A or B).
type Camera interface {
	// Model returns the identified camera model.
	Model() Model

	// Name returns the BLE advertised name.
	Name() string

	// Address returns the BLE MAC address.
	Address() string

	// TakePhoto triggers a photo capture.
	TakePhoto(ctx context.Context) error

	// StartRecording starts video recording.
	StartRecording(ctx context.Context) error

	// StopRecording stops video recording.
	StopRecording(ctx context.Context) error

	// PowerOff powers off the camera.
	PowerOff(ctx context.Context) error

	// Close disconnects from the camera.
	Close(ctx context.Context) error
}

// BatteryInfo contains battery status information.
type BatteryInfo struct {
	Level    uint8 // 0-100 percentage
	Voltage  uint16
	Charging bool
}

// StorageInfo contains storage status information.
type StorageInfo struct {
	TotalMB     uint32
	FreeMB      uint32
	FileCount   uint32
	IsFormatted bool
}

// CaptureMode represents the current camera capture mode.
type CaptureMode uint8

const (
	CaptureModePhoto      CaptureMode = 0
	CaptureModeVideo      CaptureMode = 1
	CaptureModeTimelapse  CaptureMode = 2
	CaptureModeHDR        CaptureMode = 3
	CaptureModeBulletTime CaptureMode = 4
)

func (m CaptureMode) String() string {
	switch m {
	case CaptureModePhoto:
		return "photo"
	case CaptureModeVideo:
		return "video"
	case CaptureModeTimelapse:
		return "timelapse"
	case CaptureModeHDR:
		return "hdr"
	case CaptureModeBulletTime:
		return "bullettime"
	default:
		return "unknown"
	}
}

// ParseCaptureMode converts a string to CaptureMode.
func ParseCaptureMode(s string) (CaptureMode, bool) {
	switch s {
	case "photo":
		return CaptureModePhoto, true
	case "video":
		return CaptureModeVideo, true
	case "timelapse":
		return CaptureModeTimelapse, true
	case "hdr":
		return CaptureModeHDR, true
	case "bullettime":
		return CaptureModeBulletTime, true
	default:
		return 0, false
	}
}
