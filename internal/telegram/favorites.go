package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"locationbot/internal/catalog"
)

// favoriteStore keeps each Telegram user's saved station IDs in the runtime
// volume, so favorites survive container rebuilds and restarts.
type favoriteStore struct {
	mu     sync.RWMutex
	path   string
	byUser map[string][]string
}

func openFavoriteStore(path string) (*favoriteStore, error) {
	s := &favoriteStore{path: path, byUser: make(map[string][]string)}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read favorites: %w", err)
	}
	if err := json.Unmarshal(data, &s.byUser); err != nil {
		return nil, fmt.Errorf("decode favorites: %w", err)
	}
	if s.byUser == nil {
		s.byUser = make(map[string][]string)
	}
	for userID, ids := range s.byUser {
		id, err := strconv.ParseInt(userID, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid Telegram user ID in favorites: %q", userID)
		}
		seen := make(map[string]bool, len(ids))
		for _, stationID := range ids {
			if !catalog.ValidID(stationID) || seen[stationID] {
				return nil, fmt.Errorf("invalid or duplicate station ID in favorites for user %s: %q", userID, stationID)
			}
			seen[stationID] = true
		}
	}
	return s, nil
}

func (s *favoriteStore) IDs(userID int64) []string {
	if s == nil || userID <= 0 {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.byUser[strconv.FormatInt(userID, 10)]...)
}

func (s *favoriteStore) Contains(userID int64, stationID string) bool {
	for _, id := range s.IDs(userID) {
		if id == stationID {
			return true
		}
	}
	return false
}

// Toggle returns true when the station was added and false when it was removed.
func (s *favoriteStore) Toggle(userID int64, stationID string) (bool, error) {
	if s == nil {
		return false, errors.New("favorites are not initialized")
	}
	if userID <= 0 || !catalog.ValidID(stationID) {
		return false, errors.New("invalid user or station ID")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strconv.FormatInt(userID, 10)
	current := s.byUser[key]
	nextIDs := make([]string, 0, len(current)+1)
	added := true
	for _, id := range current {
		if id == stationID {
			added = false
			continue
		}
		nextIDs = append(nextIDs, id)
	}
	if added {
		nextIDs = append(nextIDs, stationID)
	}
	return added, s.commitUserLocked(key, nextIDs)
}

func (s *favoriteStore) Remove(userID int64, stationID string) (bool, error) {
	if s == nil {
		return false, errors.New("favorites are not initialized")
	}
	if userID <= 0 || !catalog.ValidID(stationID) {
		return false, errors.New("invalid user or station ID")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := strconv.FormatInt(userID, 10)
	current := s.byUser[key]
	nextIDs := make([]string, 0, len(current))
	for _, id := range current {
		if id != stationID {
			nextIDs = append(nextIDs, id)
		}
	}
	if len(nextIDs) == len(current) {
		return false, nil
	}
	if err := s.commitUserLocked(key, nextIDs); err != nil {
		return false, err
	}
	return true, nil
}

func (s *favoriteStore) commitUserLocked(key string, ids []string) error {
	next := make(map[string][]string, len(s.byUser)+1)
	for userID, current := range s.byUser {
		next[userID] = append([]string(nil), current...)
	}
	if len(ids) == 0 {
		delete(next, key)
	} else {
		next[key] = append([]string(nil), ids...)
	}
	if err := s.write(next); err != nil {
		return err
	}
	s.byUser = next
	return nil
}

func (s *favoriteStore) write(data map[string][]string) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".favorites-*.json")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(content)
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
