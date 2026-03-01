package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChunkForBLE(t *testing.T) {
	tests := []struct {
		name      string
		msgLen    int
		maxSize   int
		wantCount int
	}{
		{"empty", 0, 20, 0},
		{"fits in one packet", 15, 20, 1},
		{"exact one packet", 20, 20, 1},
		{"two packets", 25, 20, 2},
		{"exact two packets", 40, 20, 2},
		{"three packets", 55, 20, 3},
		{"single byte", 1, 20, 1},
		{"custom max size", 50, 10, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := make([]byte, tt.msgLen)
			for i := range msg {
				msg[i] = byte(i % 256)
			}

			chunks := ChunkForBLE(msg, tt.maxSize)
			assert.Equal(t, tt.wantCount, len(chunks))

			// Verify each chunk is at most maxSize bytes.
			for i, c := range chunks {
				assert.LessOrEqual(t, len(c), tt.maxSize, "chunk %d too large", i)
				assert.Greater(t, len(c), 0, "chunk %d is empty", i)
			}

			// Verify reassembly.
			if tt.msgLen > 0 {
				reassembled := Reassemble(chunks)
				assert.Equal(t, msg, reassembled)
			}
		})
	}
}

func TestChunkForBLEDefaultMaxSize(t *testing.T) {
	msg := make([]byte, 45)
	chunks := ChunkForBLE(msg, 0) // Should use default BLEMaxPacketSize (20)
	assert.Equal(t, 3, len(chunks))
	assert.Equal(t, 20, len(chunks[0]))
	assert.Equal(t, 20, len(chunks[1]))
	assert.Equal(t, 5, len(chunks[2]))
}

func TestReassembleEmpty(t *testing.T) {
	result := Reassemble(nil)
	assert.Equal(t, []byte{}, result)
}

func TestReassemblePreservesData(t *testing.T) {
	chunks := [][]byte{
		{0x01, 0x02, 0x03},
		{0x04, 0x05},
		{0x06},
	}
	expected := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	assert.Equal(t, expected, Reassemble(chunks))
}

func TestChunkForBLEDataIntegrity(t *testing.T) {
	// Create a message with known pattern.
	msg := make([]byte, 50)
	for i := range msg {
		msg[i] = byte(i)
	}

	chunks := ChunkForBLE(msg, 20)
	reassembled := Reassemble(chunks)
	assert.Equal(t, msg, reassembled)

	// Verify chunks don't share underlying arrays (are independent copies).
	if len(chunks) > 0 {
		chunks[0][0] = 0xFF
		assert.Equal(t, byte(0), msg[0], "chunk should be independent copy")
	}
}
