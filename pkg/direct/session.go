package direct

import "sync"

// Session manages the sequence number for Architecture B messages.
// Sequence numbers cycle from 1 to 254 (0 and 255 are reserved).
type Session struct {
	mu  sync.Mutex
	seq uint8
}

// NewSession creates a new session with the initial sequence number.
func NewSession() *Session {
	return &Session{seq: 0}
}

// NextSequence returns the next sequence number, cycling 1-254.
func (s *Session) NextSequence() uint8 {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	if s.seq == 0 || s.seq == 255 {
		s.seq = 1
	}
	return s.seq
}

// CurrentSequence returns the current sequence number without advancing.
func (s *Session) CurrentSequence() uint8 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seq
}
