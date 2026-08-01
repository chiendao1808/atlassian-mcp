package auth

import (
	"errors"
	"sync/atomic"
)

var ErrNotAuthenticated = errors.New("jira not authenticated")

type SessionStore struct {
	value atomic.Value
}

func NewSessionStore() *SessionStore {
	return &SessionStore{}
}

func (s *SessionStore) Snapshot() (Credential, error) {
	v := s.value.Load()
	if v == nil {
		return Credential{}, ErrNotAuthenticated
	}
	return v.(Credential), nil
}

func (s *SessionStore) Replace(credential Credential) {
	s.value.Store(credential)
}

func (s *SessionStore) ReplaceAfter(candidate Credential, validate func(Credential) error) error {
	if err := validate(candidate); err != nil {
		return err
	}
	s.Replace(candidate)
	return nil
}
