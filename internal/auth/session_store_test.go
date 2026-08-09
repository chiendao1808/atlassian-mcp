package auth

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestStoreSnapshotsAndPreservesOnFailedReplacement(t *testing.T) {
	store := NewSessionStore()
	if _, err := store.Snapshot(); err != ErrNotAuthenticated {
		t.Fatalf("initial Snapshot error = %v", err)
	}

	old := NewCredential("alice", "old-secret")
	store.Replace(old)
	snap, err := store.Snapshot()
	if err != nil || snap.Username() != "alice" || snap.Password() != "old-secret" {
		t.Fatalf("snapshot=%+v err=%v", snap, err)
	}

	if strings.Contains(fmt.Sprint(snap), "old-secret") {
		t.Fatal("credential string formatting exposed password")
	}

	candidate := NewCredential("alice", "new-secret")
	if err := store.ReplaceAfter(candidate, func(Credential) error {
		return fmt.Errorf("candidate rejected")
	}); err == nil {
		t.Fatal("expected replacement validation error")
	}
	snap, _ = store.Snapshot()
	if snap.Password() != "old-secret" {
		t.Fatal("failed replacement should preserve old credential")
	}
}

func TestStoreConcurrentSnapshotsDuringReplacement(t *testing.T) {
	store := NewSessionStore()
	store.Replace(NewCredential("alice", "one"))
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				if _, err := store.Snapshot(); err != nil {
					t.Errorf("Snapshot error = %v", err)
				}
			}
		}()
	}
	store.Replace(NewCredential("alice", "two"))
	wg.Wait()
}

func TestReplaceIfUnchangedSkipsStaleReplacement(t *testing.T) {
	store := NewSessionStore()
	if !store.ReplaceIfUnchanged(NewCredential("alice", "one"), nil) {
		t.Fatal("empty store should accept nil expected credential")
	}
	initial, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	store.Replace(NewCredential("bob", "two"))
	if store.ReplaceIfUnchanged(NewCredential("carol", "three"), &initial) {
		t.Fatal("stale expected credential should not replace newer session")
	}
	snap, err := store.Snapshot()
	if err != nil || snap.Username() != "bob" {
		t.Fatalf("snapshot=%+v err=%v", snap, err)
	}
}
