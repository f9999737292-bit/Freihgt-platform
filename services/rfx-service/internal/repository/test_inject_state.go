package repository

import "sync"

// testInjectState coordinates one-shot integration-test injections across WithTx clones.
type testInjectState struct {
	mu   sync.Mutex
	fail bool
}

func (s *testInjectState) set(enabled bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.fail = enabled
	s.mu.Unlock()
}

func (s *testInjectState) consume() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.fail {
		return false
	}
	s.fail = false
	return true
}
