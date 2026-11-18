package catalog

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The curated catalog is controlled by the bundled dataset. Administrator-created
// locations live in a separate runtime file so synchronization, git pull and
// rebuilding the container cannot remove them.
const AdminLocationsFilename = "admin_locations.json"
const adminLocationPrefix = "custom-"

func IsAdminLocationID(id string) bool {
	return strings.HasPrefix(id, adminLocationPrefix) && ValidID(id)
}

func NewAdminLocationID() (string, error) {
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return adminLocationPrefix + hex.EncodeToString(nonce), nil
}

func (s *Store) AdminLocationsPath() string { return filepath.Join(s.dataDir, AdminLocationsFilename) }

func ValidateAdminLocations(entries map[string]Location) error {
	if len(entries) > 500 {
		return errors.New("достигнут лимит 500 дополнительных локаций")
	}
	for id, loc := range entries {
		if !IsAdminLocationID(id) || loc.ID != id {
			return fmt.Errorf("некорректный идентификатор дополнительной локации: %q", id)
		}
		if err := Revision(loc).Validate(); err != nil {
			return fmt.Errorf("локация %s: %w", id, err)
		}
	}
	return nil
}

func ReadAdminLocations(path string) (map[string]Location, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Location{}, nil
	}
	if err != nil {
		return nil, err
	}
	var locations map[string]Location
	if err := json.Unmarshal(raw, &locations); err != nil {
		return nil, fmt.Errorf("invalid admin locations file: %w", err)
	}
	if locations == nil {
		return nil, errors.New("admin locations must be a JSON object")
	}
	if err := ValidateAdminLocations(locations); err != nil {
		return nil, err
	}
	return locations, nil
}

func (s *Store) appendAdminLocations(loc []Location) ([]Location, error) {
	additions, err := ReadAdminLocations(s.AdminLocationsPath())
	if err != nil {
		return nil, err
	}
	present := make(map[string]bool, len(loc))
	for _, entry := range loc {
		present[entry.ID] = true
	}
	for id, entry := range additions {
		if present[id] {
			return nil, fmt.Errorf("additional location %s duplicates a curated ID", id)
		}
		present[id] = true
		loc = append(loc, entry)
	}
	return loc, nil
}

// CreateAdminLocation publishes one draft atomically and immediately updates
// the in-memory catalog. Drafts are not visible to users until this succeeds.
func (s *Store) CreateAdminLocation(loc Location) error {
	if !IsAdminLocationID(loc.ID) {
		return errors.New("неверный идентификатор новой локации")
	}
	if err := Revision(loc).Validate(); err != nil {
		return err
	}
	// Image metadata can only be populated through the regular sync pipeline.
	loc.ImageURL, loc.ImageExt, loc.Filename = "", "", ""
	s.adminMu.Lock()
	defer s.adminMu.Unlock()
	if _, exists := s.Find(loc.ID); exists {
		return errors.New("такая локация уже существует")
	}
	entries, err := ReadAdminLocations(s.AdminLocationsPath())
	if err != nil {
		return err
	}
	if len(entries) >= 500 {
		return errors.New("достигнут лимит 500 дополнительных локаций")
	}
	if _, exists := entries[loc.ID]; exists {
		return errors.New("такая локация уже существует")
	}
	oldData, oldErr := os.ReadFile(s.AdminLocationsPath())
	if oldErr != nil && !errors.Is(oldErr, os.ErrNotExist) {
		return oldErr
	}
	entries[loc.ID] = loc
	raw, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	if err := writePrivateAtomic(s.AdminLocationsPath(), raw); err != nil {
		return err
	}
	if err := s.Reload(); err != nil {
		// Restore the prior file if catalog reloading fails for any reason.
		if errors.Is(oldErr, os.ErrNotExist) {
			_ = os.Remove(s.AdminLocationsPath())
		} else {
			_ = writePrivateAtomic(s.AdminLocationsPath(), oldData)
		}
		_ = s.Reload()
		return fmt.Errorf("не удалось добавить локацию: %w", err)
	}
	return nil
}
