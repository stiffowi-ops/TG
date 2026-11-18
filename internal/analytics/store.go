package analytics

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	View            = "view"
	FavoriteAdded   = "favorite_add"
	FavoriteRemoved = "favorite_remove"
)

type Event struct {
	At         time.Time `json:"at"`
	UserID     int64     `json:"user_id"`
	LocationID string    `json:"location_id"`
	Kind       string    `json:"kind"`
}

type viewKey struct {
	userID     int64
	locationID string
}

type Store struct {
	lastViews map[viewKey]time.Time
	mu        sync.RWMutex
	path      string
	events    []Event
}

func Open(path string) (*Store, error) {
	s := &Store{path: path, lastViews: make(map[viewKey]time.Time)}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		var e Event
		if err = json.Unmarshal(scanner.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("malformed analytics: %w", err)
		}
		if err = e.Validate(); err != nil {
			return nil, err
		}
		s.cacheEvent(e)
	}
	return s, scanner.Err()
}
func (e Event) Validate() error {
	if e.UserID <= 0 || len(e.LocationID) < 1 || len(e.LocationID) > 80 || e.At.IsZero() {
		return errors.New("invalid analytics event")
	}
	switch e.Kind {
	case View, FavoriteAdded, FavoriteRemoved:
		return nil
	default:
		return errors.New("invalid analytics event kind")
	}
}

// Keep the most recent accepted opening for each client and card. Restored
// log entries can be out of order, so rebuild the index by timestamp.
func (s *Store) cacheEvent(e Event) {
	s.events = append(s.events, e)
	if e.Kind == View {
		key := viewKey{e.UserID, e.LocationID}
		if e.At.After(s.lastViews[key]) {
			s.lastViews[key] = e.At
		}
	}
}

func (s *Store) Record(id int64, locID, kind string, at time.Time) error {
	if s == nil {
		return nil
	}
	e := Event{At: at.UTC(), UserID: id, LocationID: locID, Kind: kind}
	if err := e.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if kind == View {
		last, ok := s.lastViews[viewKey{id, locID}]
		if ok && e.At.Sub(last).Abs() <= 20*time.Second {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	raw, _ := json.Marshal(e)
	_, err = f.Write(append(raw, '\n'))
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	s.cacheEvent(e)
	return nil
}

type Count struct {
	LocationID       string
	Views            int
	UniqueViews      int
	Additions        int
	Removals         int
	CurrentFavorites int
	Leads            int
}

func (s *Store) Snapshot() []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Event(nil), s.events...)
}
func (s *Store) Rankings(now time.Time, days int, kind string, allIDs []string, current map[string]int, leads map[string][]time.Time) []Count {
	scores := map[string]*Count{}
	for _, id := range allIDs {
		scores[id] = &Count{LocationID: id, CurrentFavorites: current[id]}
	}
	cutoff := time.Time{}
	if days > 0 {
		cutoff = now.AddDate(0, 0, -days)
	}
	unique := map[string]map[int64]struct{}{}
	for _, event := range s.Snapshot() {
		if event.At.After(now) || (!cutoff.IsZero() && event.At.Before(cutoff)) {
			continue
		}
		record := scores[event.LocationID]
		if record == nil {
			continue
		}
		switch event.Kind {
		case View:
			record.Views++
			if unique[event.LocationID] == nil {
				unique[event.LocationID] = map[int64]struct{}{}
			}
			unique[event.LocationID][event.UserID] = struct{}{}
		case FavoriteAdded:
			record.Additions++
		case FavoriteRemoved:
			record.Removals++
		}
	}
	for id, seen := range unique {
		scores[id].UniqueViews = len(seen)
	}
	for id, dates := range leads {
		r := scores[id]
		if r == nil {
			continue
		}
		for _, d := range dates {
			if !d.After(now) && (cutoff.IsZero() || !d.Before(cutoff)) {
				r.Leads++
			}
		}
	}
	output := make([]Count, 0, len(scores))
	for _, r := range scores {
		output = append(output, *r)
	}
	score := func(r Count) int {
		switch kind {
		case "views":
			return r.Views
		case "unique":
			return r.UniqueViews
		case "added":
			return r.Additions
		case "saved":
			return r.CurrentFavorites
		case "leads":
			return r.Leads
		default:
			return r.Views
		}
	}
	sort.Slice(output, func(i, j int) bool {
		a, b := score(output[i]), score(output[j])
		if a != b {
			return a > b
		}
		return strings.Compare(output[i].LocationID, output[j].LocationID) < 0
	})
	if len(output) > 10 {
		return output[:10]
	}
	return output
}
func (s *Store) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := Open(s.path)
	if err != nil {
		return err
	}
	s.events, s.lastViews = n.events, n.lastViews
	return nil
}
func (s *Store) Path() string { return s.path }

// WithExclusiveFile serializes an external on-disk restore or snapshot against
// Record. The in-memory event cache is reloaded from disk before unlocking,
// even when the external operation reports an error. Callers must not call
// another Store method from fn (non-reentrant lock).
func (s *Store) WithExclusiveFile(fn func() error) error {
	if s == nil {
		return fn()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := fn()
	fresh, reloadErr := Open(s.path)
	if reloadErr == nil {
		s.events, s.lastViews = fresh.events, fresh.lastViews
	}
	return errors.Join(err, reloadErr)
}
