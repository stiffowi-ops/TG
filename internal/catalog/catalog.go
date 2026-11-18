package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type Location struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Filename       string   `json:"filename"`
	Category       string   `json:"category"`
	Description    string   `json:"description,omitempty"`
	Rating         string   `json:"rating,omitempty"`
	Landmarks      string   `json:"landmarks,omitempty"`
	LineStatus     string   `json:"line_status,omitempty"`
	Traffic        string   `json:"traffic,omitempty"`
	Payers         string   `json:"payers,omitempty"`
	Opportunity    string   `json:"opportunity,omitempty"`
	Conclusion     string   `json:"conclusion,omitempty"`
	MapURL         string   `json:"map_url,omitempty"`
	Latitude       *float64 `json:"lat,omitempty"`
	Longitude      *float64 `json:"lon,omitempty"`
	ImageExt       string   `json:"image_ext,omitempty"`
	ImageURL       string   `json:"image_url,omitempty"`
	SourceModified string   `json:"source_modified,omitempty"`
 Sections []CardSection `json:"sections,omitempty"`
}

type Override struct {
	Title       *string  `json:"title,omitempty"`
	Category    *string  `json:"category,omitempty"`
	Description *string  `json:"description,omitempty"`
	Rating      *string  `json:"rating,omitempty"`
	Landmarks   *string  `json:"landmarks,omitempty"`
	LineStatus  *string  `json:"line_status,omitempty"`
	Traffic     *string  `json:"traffic,omitempty"`
	Payers      *string  `json:"payers,omitempty"`
	Opportunity *string  `json:"opportunity,omitempty"`
	Conclusion  *string  `json:"conclusion,omitempty"`
	MapURL      *string  `json:"map_url,omitempty"`
	Latitude    *float64 `json:"lat,omitempty"`
	Longitude   *float64 `json:"lon,omitempty"`
}

type Store struct {
	mu                              sync.RWMutex
 adminMu sync.Mutex
	locations                       []Location
	dataDir, seedPath, overridePath string
	useBundled bool
}

func New(dataDir, seedPath, overridePath string) (*Store, error) {
	s := &Store{dataDir: dataDir, seedPath: seedPath, overridePath: overridePath}
	if err := os.MkdirAll(filepath.Join(dataDir, "images"), 0755); err != nil {
		return nil, err
	}
	return s, s.Reload()
}

// NewWithBundled includes the curated 114 locations without losing existing
// entries and user overrides stored in the runtime volume.
func NewWithBundled(dataDir, seedPath, overridePath string) (*Store, error) {
	s, err := New(dataDir, seedPath, overridePath)
	if err != nil { return nil, err }
	s.useBundled = true
	if err = s.Reload(); err != nil { return nil, err }
	return s, nil
}

func (s *Store) BasePath() string { return filepath.Join(s.dataDir, "catalog.json") }
func (s *Store) DataDir() string  { return s.dataDir }

func ReadFile(path string) ([]Location, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var loc []Location
	if err = json.Unmarshal(data, &loc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return loc, nil
}

func (s *Store) Base() ([]Location, error) {
	items, err := ReadFile(s.BasePath())
	if err == nil {
		return items, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return ReadFile(s.seedPath)
}

func (s *Store) SaveBase(loc []Location) error {
	sort.Slice(loc, func(i, j int) bool { return loc[i].ID < loc[j].ID })
	b, err := json.MarshalIndent(loc, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.dataDir, ".catalog-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Chmod(0644); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), s.BasePath())
}

func (s *Store) Reload() error {
	loc, err := s.Base()
	if err != nil {
		return err
	}
	if s.useBundled {
		defaults, err := bundledLocations()
		if err != nil { return fmt.Errorf("bundled locations: %w", err) }
		// Restrict the visible catalog to the 114 curated IDs. An old Docker
		// volume or a Disk synchronization may contain extra/duplicate records.
		// Preserve existing metadata for matching approved IDs only.
		previous := make(map[string]Location, len(loc))
		for _, entry := range loc {
			if _, exists := previous[entry.ID]; !exists {
				previous[entry.ID] = entry
			}
		}
		approved := make([]Location, 0, len(defaults))
		seen := make(map[string]bool, len(defaults))
		for _, item := range defaults {
			if !validID(item.ID) || seen[item.ID] {
				return fmt.Errorf("invalid or duplicate curated ID %q", item.ID)
			}
			seen[item.ID] = true
			if saved, ok := previous[item.ID]; ok {
				// Keep photos, coordinates, links and saved fields, but prefer
				// the approved names and categories over stale synced copies.
				if saved.Filename == "" { saved.Filename = item.Filename }
				if saved.ImageExt == "" { saved.ImageExt = item.ImageExt }
				saved.Title = item.Title
				saved.Category = item.Category
				item = saved
			}
			approved = append(approved, item)
		}
		loc = approved
	}
	geo := map[string]Override{}
	if s.useBundled {
		geo, err = bundledGeoOverrides()
		if err != nil { return fmt.Errorf("bundled geo: %w", err) }
		approvedIDs := make(map[string]bool, len(loc))
		for _, item := range loc { approvedIDs[item.ID] = true }
		for id := range geo {
			if !approvedIDs[id] { return fmt.Errorf("geo ID %s not in curated catalog", id) }
		}
	}
	templates := map[string]Override{}
	if s.useBundled {
		templates, err = bundledPartnerCards()
		if err != nil { return fmt.Errorf("bundled partner cards: %w", err) }
	}
	// Shipped partner cards are kept separate from mutable user overrides so
	// upgrading the bot never wipes locally customized coordinates or texts.
	partnerPath := filepath.Join(filepath.Dir(s.overridePath), "partner_cards.json")
	b, err := os.ReadFile(partnerPath)
	if err == nil {
		// Optional local partner cards override the bundled default by station ID.
		fileTemplates := map[string]Override{}
		if err = json.Unmarshal(b, &fileTemplates); err != nil {
			return fmt.Errorf("invalid partner cards JSON: %w", err)
		}
		for id, entry := range fileTemplates { templates[id] = entry }
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	override := map[string]Override{}
	b, err = os.ReadFile(s.overridePath)
	if err == nil {
		if err = json.Unmarshal(b, &override); err != nil {
			return fmt.Errorf("invalid overrides JSON: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for i := range loc {
		l := &loc[i]
		l.ImageURL = ""
		for _, layer := range []map[string]Override{geo, templates, override} {
			if o, ok := layer[l.ID]; ok {
				if o.Title != nil {
					l.Title = *o.Title
				}
				if o.Category != nil {
					l.Category = *o.Category
				}
				if o.Description != nil {
					l.Description = *o.Description
				}
				if o.Rating != nil {
					l.Rating = *o.Rating
				}
				if o.Landmarks != nil {
					l.Landmarks = *o.Landmarks
				}
				if o.LineStatus != nil {
					l.LineStatus = *o.LineStatus
				}
				if o.Traffic != nil {
					l.Traffic = *o.Traffic
				}
				if o.Payers != nil {
					l.Payers = *o.Payers
				}
				if o.Opportunity != nil {
					l.Opportunity = *o.Opportunity
				}
				if o.Conclusion != nil {
					l.Conclusion = *o.Conclusion
				}
				if o.MapURL != nil {
					l.MapURL = *o.MapURL
				}
				if o.Latitude != nil && *o.Latitude >= -90 && *o.Latitude <= 90 {
					l.Latitude = o.Latitude
				}
				if o.Longitude != nil && *o.Longitude >= -180 && *o.Longitude <= 180 {
					l.Longitude = o.Longitude
				}
			}
		}
		if l.ImageExt == "" {
			l.ImageExt = strings.ToLower(filepath.Ext(l.Filename))
		}
		if validID(l.ID) && validExt(l.ImageExt) {
			name := l.ID + l.ImageExt
			if st, e := os.Stat(filepath.Join(s.dataDir, "images", name)); e == nil && st.Mode().IsRegular() {
				l.ImageURL = "/media/" + name
			}
		}
	}
	loc, err = s.appendAdminLocations(loc)
 if err != nil { return err }
 if err := s.applyAdminCards(loc); err != nil { return err }
 sort.Slice(loc, func(i, j int) bool { return loc[i].ID < loc[j].ID })
	s.mu.Lock()
	s.locations = loc
	s.mu.Unlock()
	return nil
}

func validID(id string) bool {
	if id == "" || len(id) > 80 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}
func ValidID(id string) bool { return validID(id) }
func validExt(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
		return true
	}
	return false
}
func ValidExt(ext string) bool { return validExt(ext) }

func (s *Store) List() []Location {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Location, len(s.locations))
	copy(out, s.locations)
	return out
}
func (s *Store) Find(id string) (Location, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, l := range s.locations {
		if l.ID == id {
			return l, true
		}
	}
	return Location{}, false
}

func Normalize(s string) string {
	s = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), "ё", "е"))
	for _, p := range []string{"метро", "станция", "м.", "№", "«", "»", "\""} {
		s = strings.ReplaceAll(s, p, " ")
	}
	return strings.Join(strings.Fields(s), " ")
}

func Search(locs []Location, text string) []Location {
	needle := Normalize(text)
	if needle == "" {
		return locs
	}
	words := strings.Fields(needle)
	var found []Location
	for _, l := range locs {
		field := Normalize(l.Title + " " + l.Category + " " + l.Description)
		hit := true
		for _, w := range words {
			if !strings.Contains(field, w) {
				hit = false
				break
			}
		}
		if hit {
			found = append(found, l)
		}
	}
	return found
}
