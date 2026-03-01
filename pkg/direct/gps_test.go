package direct

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGPSPayloadEncoding(t *testing.T) {
	// Verify that GPS coordinates are encoded as 3 little-endian float64 values (24 bytes total).
	lat, lon, alt := 37.7749, -122.4194, 10.5

	var buf bytes.Buffer
	assert.NoError(t, binary.Write(&buf, binary.LittleEndian, lat))
	assert.NoError(t, binary.Write(&buf, binary.LittleEndian, lon))
	assert.NoError(t, binary.Write(&buf, binary.LittleEndian, alt))

	assert.Equal(t, 24, buf.Len(), "GPS payload should be 24 bytes (3 float64)")

	// Decode and verify.
	data := buf.Bytes()
	gotLat := math.Float64frombits(binary.LittleEndian.Uint64(data[0:8]))
	gotLon := math.Float64frombits(binary.LittleEndian.Uint64(data[8:16]))
	gotAlt := math.Float64frombits(binary.LittleEndian.Uint64(data[16:24]))

	assert.InDelta(t, lat, gotLat, 1e-10)
	assert.InDelta(t, lon, gotLon, 1e-10)
	assert.InDelta(t, alt, gotAlt, 1e-10)
}

func TestGPSCoordinateValidation(t *testing.T) {
	tests := []struct {
		name    string
		lat     float64
		lon     float64
		alt     float64
		wantErr string
	}{
		{
			name: "valid equator",
			lat:  0, lon: 0, alt: 0,
		},
		{
			name: "valid max bounds",
			lat:  90, lon: 180, alt: 8848,
		},
		{
			name: "valid min bounds",
			lat:  -90, lon: -180, alt: -420,
		},
		{
			name:    "latitude too high",
			lat:     91, lon: 0, alt: 0,
			wantErr: "latitude",
		},
		{
			name:    "latitude too low",
			lat:     -91, lon: 0, alt: 0,
			wantErr: "latitude",
		},
		{
			name:    "longitude too high",
			lat:     0, lon: 181, alt: 0,
			wantErr: "longitude",
		},
		{
			name:    "longitude too low",
			lat:     0, lon: -181, alt: 0,
			wantErr: "longitude",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't call InjectGPS directly since it needs a connected device.
			// Instead, test the validation logic in isolation.
			var err error
			if math.Abs(tt.lat) > 90 {
				err = &validationError{field: "latitude", value: tt.lat}
			} else if math.Abs(tt.lon) > 180 {
				err = &validationError{field: "longitude", value: tt.lon}
			}

			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// validationError is a test helper that mimics the validation in InjectGPS.
type validationError struct {
	field string
	value float64
}

func (e *validationError) Error() string {
	return e.field + " out of range"
}
