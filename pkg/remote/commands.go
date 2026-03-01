package remote

import "github.com/xaionaro-go/insta360ctl/pkg/protocol"

// Shutter triggers the camera shutter (take photo or start/stop recording
// depending on current mode).
func (s *Server) Shutter() error {
	return s.SendCommand(protocol.RemoteCommandShutter)
}

// CycleMode cycles through the camera's capture modes.
func (s *Server) CycleMode() error {
	return s.SendCommand(protocol.RemoteCommandMode)
}

// WakeScreen turns on the camera's display screen.
func (s *Server) WakeScreen() error {
	return s.SendCommand(protocol.RemoteCommandScreen)
}

// PowerOff powers off the camera.
func (s *Server) PowerOff() error {
	return s.SendCommand(protocol.RemoteCommandPowerOff)
}
