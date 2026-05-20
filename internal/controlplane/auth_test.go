package controlplane

import (
	"context"
	"testing"
	"time"
)

type memoryAuthStateStore struct {
	state authState
}

func (s *memoryAuthStateStore) Load(context.Context) (authState, error) {
	return normalizeAuthState(s.state), nil
}

func (s *memoryAuthStateStore) Save(_ context.Context, state authState) error {
	s.state = normalizeAuthState(state)
	return nil
}

func TestManagerAuthStateStoreSupportsManualLockRelease(t *testing.T) {
	t.Parallel()

	store := &memoryAuthStateStore{}
	manager := NewManager(NewFileStore(""))
	manager.SetAuthStateStore(store)
	manager.SetAuthConfig("issue7-token", time.Hour, 2)

	if _, _, err := manager.Login(context.Background(), "wrong", "127.0.0.1"); err != ErrControlPlaneAuthInvalidToken {
		t.Fatalf("first login error = %v, want %v", err, ErrControlPlaneAuthInvalidToken)
	}
	if _, _, err := manager.Login(context.Background(), "wrong", "127.0.0.1"); err != ErrControlPlaneAuthLocked {
		t.Fatalf("second login error = %v, want %v", err, ErrControlPlaneAuthLocked)
	}
	locked, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("store.Load() error = %v", err)
	}
	if !locked.Locks["127.0.0.1"].Locked {
		t.Fatalf("locked state = %#v, want locked IP", locked.Locks)
	}

	// Simulate the documented first-phase recovery path: one operator clears the lock in persistence.
	delete(locked.Locks, "127.0.0.1")
	if err := store.Save(context.Background(), locked); err != nil {
		t.Fatalf("store.Save() error = %v", err)
	}

	status, sessionID, err := manager.Login(context.Background(), "issue7-token", "127.0.0.1")
	if err != nil {
		t.Fatalf("login after manual release error = %v", err)
	}
	if !status.Authenticated || sessionID == "" {
		t.Fatalf("login after manual release = %#v, %q; want authenticated session", status, sessionID)
	}
}
