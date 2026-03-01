package direct

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xaionaro-go/insta360ctl/pkg/camera"
	"github.com/xaionaro-go/insta360ctl/pkg/protocol"
	"github.com/xaionaro-go/insta360ctl/pkg/protocol/messagecode"
)

func TestParseGo3BatteryProtobuf(t *testing.T) {
	tests := []struct {
		name     string
		pb       []byte
		level    uint8
		charging bool
	}{
		{
			name:     "100% charging",
			pb:       []byte{0x08, 0x64, 0x10, 0x01}, // field1=100, field2=1
			level:    100,
			charging: true,
		},
		{
			name:     "50% not charging",
			pb:       []byte{0x08, 0x32, 0x10, 0x00}, // field1=50, field2=0
			level:    50,
			charging: false,
		},
		{
			name:     "0% not charging",
			pb:       []byte{0x08, 0x00}, // field1=0, no field2
			level:    0,
			charging: false,
		},
		{
			name:     "level only, no charging field",
			pb:       []byte{0x08, 0x4B}, // field1=75
			level:    75,
			charging: false,
		},
		{
			name:     "varint multi-byte level",
			pb:       []byte{0x08, 0xC8, 0x01}, // field1=200 (varint: 0xC8 0x01)
			level:    200,
			charging: false,
		},
		{
			name:     "empty payload",
			pb:       []byte{},
			level:    0,
			charging: false,
		},
		{
			name:     "nil payload",
			pb:       nil,
			level:    0,
			charging: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, charging := parseGo3BatteryProtobuf(tt.pb)
			assert.Equal(t, tt.level, level)
			assert.Equal(t, tt.charging, charging)
		})
	}
}

func TestExtractProtoString(t *testing.T) {
	tests := []struct {
		name     string
		pb       []byte
		expected string
	}{
		{
			name:     "simple string",
			pb:       []byte{0x12, 0x05, 'h', 'e', 'l', 'l', 'o'},
			expected: "hello",
		},
		{
			name:     "error message",
			pb:       []byte{0x08, 0x01, 0x12, 0x10, 'm', 's', 'g', ' ', 'e', 'x', 'e', 'c', 'u', 't', 'e', ' ', 'e', 'r', 'r', '.'},
			expected: "msg execute err.",
		},
		{
			name:     "no field 2",
			pb:       []byte{0x08, 0x01},
			expected: "(raw: 0801)",
		},
		{
			name:     "empty string",
			pb:       []byte{0x12, 0x00},
			expected: "",
		},
		{
			name:     "truncated string",
			pb:       []byte{0x12, 0x10, 'h', 'i'}, // declares 16 bytes but only 2 available
			expected: "(raw: 12106869)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractProtoString(tt.pb)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractPayloadGo2StatusCodes(t *testing.T) {
	dev := &Device{
		Cam: camera.ModelGo3,
	}

	tests := []struct {
		name        string
		cmd         messagecode.Code
		payload     []byte
		wantPayload []byte
		wantErr     string
	}{
		{
			name:        "200 OK with payload",
			cmd:         messagecode.CodeResponseOK,
			payload:     []byte{0x08, 0x64, 0x10, 0x01},
			wantPayload: []byte{0x08, 0x64, 0x10, 0x01},
		},
		{
			name:        "200 OK empty payload",
			cmd:         messagecode.CodeResponseOK,
			payload:     nil,
			wantPayload: []byte{},
		},
		{
			name:    "400 Bad Request with error message",
			cmd:     messagecode.CodeResponseBadRequest,
			payload: []byte{0x08, 0x01, 0x12, 0x0F, 'u', 'n', 'k', 'n', 'o', 'w', 'n', ' ', 'm', 's', 'g', ' ', 'c', 'o', 'd'},
			wantErr: "camera returned 400:",
		},
		{
			name:    "500 Execution Error",
			cmd:     messagecode.CodeResponseError,
			payload: []byte{0x08, 0x01, 0x12, 0x10, 'm', 's', 'g', ' ', 'e', 'x', 'e', 'c', 'u', 't', 'e', ' ', 'e', 'r', 'r', '.'},
			wantErr: "camera returned 500: msg execute err.",
		},
		{
			name:    "501 Not Implemented",
			cmd:     messagecode.CodeResponseNotImpl,
			payload: nil,
			wantErr: "camera returned 501: not implemented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a Go2 inner message with the status code as command code.
			resp := protocol.EncodeGo2Message(tt.cmd, 1, tt.payload)

			payload, err := dev.extractPayload(resp)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantPayload, payload)
			}
		})
	}
}

func TestExtractPayloadHeader16(t *testing.T) {
	dev := &Device{
		Cam: camera.ModelX3,
	}

	payload := []byte{0x50, 0x00, 0x10, 0x27}
	resp := protocol.EncodeMessage(messagecode.CodeGetBatteryInfo, 1, payload)

	got, err := dev.extractPayload(resp)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}

func TestNextGo2Seq(t *testing.T) {
	dev := &Device{}

	// First call starts at 1.
	assert.Equal(t, uint8(1), dev.nextGo2Seq())
	assert.Equal(t, uint8(2), dev.nextGo2Seq())
	assert.Equal(t, uint8(3), dev.nextGo2Seq())

	// Wrap around at 254.
	dev.go2SeqCounter = 253
	assert.Equal(t, uint8(254), dev.nextGo2Seq())
	assert.Equal(t, uint8(1), dev.nextGo2Seq())
	assert.Equal(t, uint8(2), dev.nextGo2Seq())
}

func TestParseGo3StorageNotification(t *testing.T) {
	tests := []struct {
		name      string
		pb        []byte
		totalMB   uint32
		freeMB    uint32
		fileCount uint32
	}{
		{
			name: "full notification with recording state",
			// 08 04 20 B8 17 28 05 30 88 0E
			// field1=4, field4=3000, field5=5, field6=1800
			pb:        []byte{0x08, 0x04, 0x20, 0xB8, 0x17, 0x28, 0x05, 0x30, 0x88, 0x0E},
			totalMB:   1800,
			freeMB:    3000,
			fileCount: 5,
		},
		{
			name: "idle state zeros",
			// 08 00 20 00 28 00 30 88 0E
			pb:        []byte{0x08, 0x00, 0x20, 0x00, 0x28, 0x00, 0x30, 0x88, 0x0E},
			totalMB:   1800,
			freeMB:    0,
			fileCount: 0,
		},
		{
			name: "post-recording state",
			// 08 01 20 B8 17 28 05 30 88 0E
			pb:        []byte{0x08, 0x01, 0x20, 0xB8, 0x17, 0x28, 0x05, 0x30, 0x88, 0x0E},
			totalMB:   1800,
			freeMB:    3000,
			fileCount: 5,
		},
		{
			name:      "empty payload",
			pb:        []byte{},
			totalMB:   0,
			freeMB:    0,
			fileCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := parseGo3StorageNotification(tt.pb)
			require.NotNil(t, info)
			assert.Equal(t, tt.totalMB, info.TotalMB)
			assert.Equal(t, tt.freeMB, info.FreeMB)
			assert.Equal(t, tt.fileCount, info.FileCount)
		})
	}
}

func TestGetStorageInfoGo3CachedData(t *testing.T) {
	dev, _ := newTestDevice(camera.ModelGo3)
	ctx := context.Background()

	// Initially, no cached data — should return error.
	_, err := dev.GetStorageInfo(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not yet available")

	// Simulate a storage notification arriving.
	dev.mu.Lock()
	dev.cachedStorageInfo = &camera.StorageInfo{
		TotalMB:   1800,
		FreeMB:    3000,
		FileCount: 5,
	}
	dev.mu.Unlock()

	// Now should return cached data.
	info, err := dev.GetStorageInfo(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint32(1800), info.TotalMB)
	assert.Equal(t, uint32(3000), info.FreeMB)
	assert.Equal(t, uint32(5), info.FileCount)
}

func TestCacheNotificationDataStorage(t *testing.T) {
	dev, _ := newTestDevice(camera.ModelGo3)
	ctx := context.Background()

	// Simulate a 0x2010 notification payload.
	// field1=4, field4=3000, field5=5, field6=1800
	payload := []byte{0x08, 0x04, 0x20, 0xB8, 0x17, 0x28, 0x05, 0x30, 0x88, 0x0E}
	dev.cacheNotificationData(ctx, messagecode.CodeGo2NotifyStorageState, payload)

	dev.mu.Lock()
	cached := dev.cachedStorageInfo
	dev.mu.Unlock()

	require.NotNil(t, cached)
	assert.Equal(t, uint32(1800), cached.TotalMB)
	assert.Equal(t, uint32(3000), cached.FreeMB)
	assert.Equal(t, uint32(5), cached.FileCount)
}

func TestNextGo2SeqFullCycle(t *testing.T) {
	dev := &Device{}

	seen := make(map[uint8]bool)
	for i := 0; i < 300; i++ {
		seq := dev.nextGo2Seq()
		assert.True(t, seq >= 1 && seq <= 254, "sequence %d out of range [1, 254]", seq)
		if i < 254 {
			assert.False(t, seen[seq], "duplicate sequence %d at iteration %d", seq, i)
		}
		seen[seq] = true
	}
}
