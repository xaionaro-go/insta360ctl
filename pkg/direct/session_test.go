package direct

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSessionNextSequence(t *testing.T) {
	s := NewSession()

	// First call should return 1.
	assert.Equal(t, uint8(1), s.NextSequence())
	assert.Equal(t, uint8(2), s.NextSequence())
	assert.Equal(t, uint8(3), s.NextSequence())
}

func TestSessionWraparound(t *testing.T) {
	s := NewSession()
	s.mu.Lock()
	s.seq = 253
	s.mu.Unlock()

	// 253 -> 254
	assert.Equal(t, uint8(254), s.NextSequence())
	// 254 -> should wrap to 1 (skipping 0 and 255)
	assert.Equal(t, uint8(1), s.NextSequence())
	// Back to normal
	assert.Equal(t, uint8(2), s.NextSequence())
}

func TestSessionCurrentSequence(t *testing.T) {
	s := NewSession()
	assert.Equal(t, uint8(0), s.CurrentSequence())

	s.NextSequence()
	assert.Equal(t, uint8(1), s.CurrentSequence())

	s.NextSequence()
	assert.Equal(t, uint8(2), s.CurrentSequence())
}

func TestSessionCycle(t *testing.T) {
	s := NewSession()

	// Run through a full cycle to verify no panics or stuck values.
	seen := make(map[uint8]bool)
	for i := 0; i < 300; i++ {
		seq := s.NextSequence()
		assert.True(t, seq >= 1 && seq <= 254, "sequence %d out of range [1, 254]", seq)
		if i < 254 {
			assert.False(t, seen[seq], "duplicate sequence %d at iteration %d", seq, i)
		}
		seen[seq] = true
	}
}
