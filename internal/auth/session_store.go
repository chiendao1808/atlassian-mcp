package auth

import (
	"errors"
	"sync"
	"sync/atomic"
)

// ErrNotAuthenticated reports that no credential has been validated for this module session.
var ErrNotAuthenticated = errors.New("not authenticated")

// SessionStore keeps the active credential for exactly one module.
type SessionStore struct {
	mu    sync.Mutex
	value atomic.Value
}

// NewSessionStore returns an empty in-memory session store.
func NewSessionStore() *SessionStore {
	return &SessionStore{}
}

// Snapshot returns the active credential, or ErrNotAuthenticated before successful auth.
func (s *SessionStore) Snapshot() (Credential, error) {
	v := s.value.Load()
	if v == nil {
		return Credential{}, ErrNotAuthenticated
	}
	return v.(Credential), nil
}

// Replace atomically swaps in a validated credential.
func (s *SessionStore) Replace(credential Credential) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value.Store(credential)
}

// ReplaceAfter validates a candidate before replacing the current credential.
func (s *SessionStore) ReplaceAfter(candidate Credential, validate func(Credential) error) error {
	if err := validate(candidate); err != nil {
		return err
	}
	s.Replace(candidate)
	return nil
}

// ReplaceIfUnchanged swaps in candidate only when the active credential still matches expected.
func (s *SessionStore) ReplaceIfUnchanged(candidate Credential, expected *Credential) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.value.Load()
	if expected == nil {
		if v != nil {
			return false
		}
	} else if v == nil || v.(Credential) != *expected {
		return false
	}
	s.value.Store(candidate)
	return true
}
