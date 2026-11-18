package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CardSection is an ordered, optional accordion in the public card.
type CardSection struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	Enabled bool   `json:"enabled"`
}

// CardRevision stores only mutable public-facing fields. The ID is immutable.
type CardRevision struct {
	Category    string        `json:"category,omitempty"`
	Description string        `json:"description,omitempty"`
	Rating      string        `json:"rating,omitempty"`
	Latitude    *float64      `json:"lat,omitempty"`
	Longitude   *float64      `json:"lon,omitempty"`
	Title       string        `json:"title"`
	LineStatus  string        `json:"line_status"`
	Landmarks   string        `json:"landmarks"`
	MapURL      string        `json:"map_url"`
	Sections    []CardSection `json:"sections"`
}

func DefaultSections(l Location) []CardSection {
	return []CardSection{
		{ID: "traffic", Title: "🚶 Трафик и локация", Body: choose(l.Traffic, l.Description), Enabled: true},
		{ID: "payers", Title: "👥 Кто платит", Body: l.Payers, Enabled: true},
		{ID: "opportunity", Title: "📈 Возможности для бизнеса", Body: l.Opportunity, Enabled: true},
		{ID: "conclusion", Title: "🎯 Почему именно здесь", Body: l.Conclusion, Enabled: true},
	}
}
func choose(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func Revision(l Location) CardRevision {
	sections := l.Sections
	if sections == nil {
		sections = DefaultSections(l)
	}
	return CardRevision{Title: l.Title, Category: l.Category, Description: l.Description, Rating: l.Rating, Latitude: l.Latitude, Longitude: l.Longitude, LineStatus: l.LineStatus, Landmarks: l.Landmarks, MapURL: l.MapURL, Sections: append([]CardSection{}, sections...)}
}

func (r CardRevision) Validate() error {
	if strings.TrimSpace(r.Title) == "" || len([]rune(r.Title)) > 150 {
		return errors.New("название должно содержать от 1 до 150 символов")
	}
	if len([]rune(r.LineStatus)) > 350 || len([]rune(r.Landmarks)) > 500 || len([]rune(r.Description)) > 3500 {
		return errors.New("статус или ориентир слишком длинный")
	}
	if len(r.MapURL) > 2000 || (r.MapURL != "" && !strings.HasPrefix(r.MapURL, "https://")) {
		return errors.New("ссылка карты должна начинаться с https://")
	}
	if (r.Latitude != nil && (*r.Latitude < -90 || *r.Latitude > 90)) || (r.Longitude != nil && (*r.Longitude < -180 || *r.Longitude > 180)) {
		return errors.New("некорректные координаты")
	}
	if len(r.Sections) > 15 {
		return errors.New("допускается не больше 15 разделов")
	}
	seen := map[string]bool{}
	for _, s := range r.Sections {
		if !ValidID(s.ID) || seen[s.ID] || len([]rune(s.Title)) < 1 || len([]rune(s.Title)) > 100 || len([]rune(s.Body)) > 3500 {
			return fmt.Errorf("неверные данные раздела %q", s.ID)
		}
		seen[s.ID] = true
	}
	return nil
}

func (s *Store) AdminCardsPath() string { return filepath.Join(s.dataDir, "admin_cards.json") }

// AdminEdit replaces the complete mutable revision for one known location.
// A separate mutex keeps concurrent administrators from losing each other's
// writes; the precondition prevents a stale draft from overwriting changes.
func (s *Store) AdminEdit(id, expected string, revision CardRevision) error {
	if !ValidID(id) {
		return errors.New("неверный ID локации")
	}
	if err := revision.Validate(); err != nil {
		return err
	}
	s.adminMu.Lock()
	defer s.adminMu.Unlock()
	before, ok := s.Find(id)
	if !ok {
		return errors.New("локация не найдена")
	}
	if expected != "" && RevisionHash(Revision(before)) != expected {
		return errors.New("карточка была изменена другим администратором; обновите черновик")
	}
	current, err := s.adminCardsOnDisk()
	if err != nil {
		return err
	}
	current[id] = revision
	data, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return err
	}
	if err = writePrivateAtomic(s.AdminCardsPath(), data); err != nil {
		return err
	}
	return s.Reload()
}

func RevisionHash(rev CardRevision) string {
	b, _ := json.Marshal(rev)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func (s *Store) adminCardsOnDisk() (map[string]CardRevision, error) {
	cards := map[string]CardRevision{}
	data, err := os.ReadFile(s.AdminCardsPath())
	if errors.Is(err, os.ErrNotExist) {
		return cards, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(data, &cards); err != nil {
		return nil, err
	}
	for id, r := range cards {
		if !ValidID(id) {
			return nil, fmt.Errorf("invalid card ID %q", id)
		}
		if err = r.Validate(); err != nil {
			return nil, fmt.Errorf("invalid card %s: %w", id, err)
		}
	}
	return cards, nil
}

func (s *Store) applyAdminCards(items []Location) error {
	cards, err := s.adminCardsOnDisk()
	if err != nil {
		return err
	}
	for i := range items {
		if r, ok := cards[items[i].ID]; ok {
			items[i].Title = r.Title
			items[i].Category = r.Category
			items[i].Description = r.Description
			items[i].Rating = r.Rating
			items[i].Latitude = r.Latitude
			items[i].Longitude = r.Longitude
			items[i].LineStatus = r.LineStatus
			items[i].Landmarks = r.Landmarks
			items[i].MapURL = r.MapURL
			items[i].Sections = append([]CardSection{}, r.Sections...)
		}
	}
	return nil
}

func writePrivateAtomic(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".karty-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(raw)
	}
	if err == nil {
		err = file.Sync()
	}
	if c := file.Close(); err == nil {
		err = c
	}
	if err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

// ReloadAdminCards is used after a verified backup has been restored.
func (s *Store) ReloadAdminCards() error { return s.Reload() }
