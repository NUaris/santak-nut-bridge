package main

import (
	"errors"
	"sync"
	"time"
)

var errNoData = errors.New("no UPS data available")

type SnapshotStore struct {
	mu       sync.RWMutex
	snapshot Snapshot
	hasData  bool
	lastErr  error
}

func (store *SnapshotStore) Update(snapshot Snapshot) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.snapshot = snapshot
	store.hasData = true
	store.lastErr = nil
}

func (store *SnapshotStore) MarkError(err error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.lastErr = err
}

func (store *SnapshotStore) Current(staleAfter time.Duration) (Snapshot, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	if !store.hasData {
		if store.lastErr != nil {
			return Snapshot{}, store.lastErr
		}
		return Snapshot{}, errNoData
	}
	if time.Since(store.snapshot.UpdatedAt) > staleAfter {
		if store.lastErr != nil {
			return Snapshot{}, store.lastErr
		}
		return Snapshot{}, errors.New("UPS data is stale")
	}
	copyValues := make(map[string]string, len(store.snapshot.Values))
	for key, value := range store.snapshot.Values {
		copyValues[key] = value
	}
	copySnapshot := store.snapshot
	copySnapshot.Values = copyValues
	return copySnapshot, nil
}
