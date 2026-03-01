package direct

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/xaionaro-go/insta360ctl/pkg/protocol/messagecode"
)

// InjectGPS sends GPS coordinates to the camera for geotagging.
// Latitude and longitude are in decimal degrees, altitude in meters.
func (d *Device) InjectGPS(ctx context.Context, lat, lon, alt float64) error {
	var buf bytes.Buffer

	// Pack as little-endian doubles (float64).
	if err := binary.Write(&buf, binary.LittleEndian, lat); err != nil {
		return fmt.Errorf("failed to encode latitude: %w", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, lon); err != nil {
		return fmt.Errorf("failed to encode longitude: %w", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, alt); err != nil {
		return fmt.Errorf("failed to encode altitude: %w", err)
	}

	// Validate coordinates.
	if math.Abs(lat) > 90 {
		return fmt.Errorf("latitude %f out of range [-90, 90]", lat)
	}
	if math.Abs(lon) > 180 {
		return fmt.Errorf("longitude %f out of range [-180, 180]", lon)
	}

	_, err := d.SendCommand(ctx, messagecode.CodeSetGPS, buf.Bytes())
	if err != nil {
		return fmt.Errorf("inject GPS failed: %w", err)
	}
	return nil
}
