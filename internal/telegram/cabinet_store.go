package telegram

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"locationbot/internal/catalog"
)

// Keep the original single location fields readable by older backups/tools.
type leadLocation struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type leadUpdate struct {
	ID         int            `json:"id"`
	Token      string         `json:"token"`
	Locations  []leadLocation `json:"locations,omitempty"`
	Comment    string         `json:"comment,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	AcceptedAt *time.Time     `json:"accepted_at,omitempty"`
	AcceptedBy int64          `json:"accepted_by,omitempty"`
}
type supplementDraft struct {
	Number    int64          `json:"number"`
	Token     string         `json:"token"`
	Locations []leadLocation `json:"locations,omitempty"`
	Comment   string         `json:"comment,omitempty"`
	Stage     string         `json:"stage"`
	Page      int            `json:"page,omitempty"`
	Paused    bool           `json:"paused,omitempty"`
}

const maxCaseSupplements = 2
const supplementLimitText = "Вы уже отправили два дополнения к этой заявке. Пожалуйста, дождитесь ответа специалиста. Статус можно проверить в разделе «Мои заявки»."
const processedText = "✅ Заявка обработана."

func (l leadApplication) supplementLimitReached() bool { return len(l.Updates) >= maxCaseSupplements }

func (l leadApplication) supplementError(now time.Time) error {
	if l.isResolved() || l.autoCloseDue(now) {
		return errors.New("Срок подачи дополнений завершён. Откройте «Мои заявки», чтобы посмотреть статус.")
	}
	if l.supplementLimitReached() {
		return errors.New(supplementLimitText)
	}
	return nil
}

func (l leadApplication) isResolved() bool { return l.ResolvedAt != nil }
func (l leadApplication) locations() []leadLocation {
	if len(l.Locations) > 0 {
		return append([]leadLocation(nil), l.Locations...)
	}
	if l.LocationID != "" {
		return []leadLocation{{ID: l.LocationID, Name: l.LocationName}}
	}
	return nil
}
func leadStatus(l leadApplication) string {
	if l.isResolved() {
		return "✅ Решено"
	}
	if l.TakenInWork {
		return "🛠 В работе"
	}
	return "📨 Получено"
}
func cloneLeadDraft(d leadDraft) leadDraft { return d }
func cloneSupplementDraft(d supplementDraft) supplementDraft {
	d.Locations = append([]leadLocation(nil), d.Locations...)
	return d
}
func cloneServiceFields(l *leadApplication) {
	l.Locations = append([]leadLocation(nil), l.Locations...)
	l.Updates = append([]leadUpdate(nil), l.Updates...)
	for i := range l.Updates {
		l.Updates[i].Locations = append([]leadLocation(nil), l.Updates[i].Locations...)
		if l.Updates[i].AcceptedAt != nil {
			v := *l.Updates[i].AcceptedAt
			l.Updates[i].AcceptedAt = &v
		}
	}
	if l.ResolvedAt != nil {
		v := *l.ResolvedAt
		l.ResolvedAt = &v
	}
	if l.NoticeMessages != nil {
		m := map[string]int{}
		for k, v := range l.NoticeMessages {
			m[k] = v
		}
		l.NoticeMessages = m
	}
	if l.NoticeReceipts != nil {
		m := make(map[string]bool, len(l.NoticeReceipts))
		for k, v := range l.NoticeReceipts {
			m[k] = v
		}
		l.NoticeReceipts = m
	}
}

func (s *leadStore) supplement(userID int64) (supplementDraft, bool) {
	if s == nil {
		return supplementDraft{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.state.Supplements[strconv.FormatInt(userID, 10)]
	return cloneSupplementDraft(d), ok
}
func (s *leadStore) putSupplement(userID int64, d supplementDraft) error {
	if s == nil || userID <= 0 || d.Token == "" {
		return errors.New("неверное дополнение")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := strconv.FormatInt(userID, 10)
	l, ok := s.state.Submitted[key]
	if !ok || l.Number != d.Number || l.isResolved() {
		return errors.New("обращение уже завершено или недоступно")
	}
	if err := l.supplementError(time.Now()); err != nil {
		return err
	}
	next := s.cloneLocked()
	next.Supplements[key] = cloneSupplementDraft(d)
	return s.saveLocked(next)
}
func newSupplement(number int64) (supplementDraft, error) {
	var token [8]byte
	if _, err := rand.Read(token[:]); err != nil {
		return supplementDraft{}, err
	}
	return supplementDraft{Number: number, Token: hex.EncodeToString(token[:]), Stage: "choose"}, nil
}

func (b *Bot) pauseClientForms(userID int64) error {
	if b.leads == nil {
		return nil
	}
	s := b.leads
	s.mu.Lock()
	defer s.mu.Unlock()
	key := strconv.FormatInt(userID, 10)
	d, hasDraft := s.state.Drafts[key]
	u, hasUpdate := s.state.Supplements[key]
	if (!hasDraft || d.Paused) && (!hasUpdate || u.Paused) {
		return nil
	}
	next := s.cloneLocked()
	if hasDraft {
		d.Paused = true
		next.Drafts[key] = d
	}
	if hasUpdate {
		u.Paused = true
		next.Supplements[key] = u
	}
	return s.saveLocked(next)
}
func (s *leadStore) cancelSupplement(userID int64, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := strconv.FormatInt(userID, 10)
	if d, ok := s.state.Supplements[key]; !ok || d.Token != token {
		return nil
	}
	next := s.cloneLocked()
	delete(next.Supplements, key)
	return s.saveLocked(next)
}

// Submission, idempotency token and all locations are committed together.
// A retry of the same Send button never creates a second update.
func (s *leadStore) submitSupplement(userID, number int64, token string) (leadApplication, bool, error) {
	if s == nil || userID <= 0 || number <= 0 || token == "" {
		return leadApplication{}, false, errors.New("неверное дополнение")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := strconv.FormatInt(userID, 10)
	l, ok := s.state.Submitted[key]
	if !ok || l.Number != number {
		return leadApplication{}, false, errors.New("обращение недоступно")
	}
	for _, u := range l.Updates {
		if u.Token == token {
			return cloneLead(l), false, nil
		}
	}
	if err := l.supplementError(time.Now()); err != nil {
		return leadApplication{}, false, err
	}
	d, ok := s.state.Supplements[key]
	if !ok || d.Number != number || d.Token != token || d.Stage != "review" {
		return leadApplication{}, false, errors.New("дополнение изменилось; проверьте его ещё раз")
	}
	comment := strings.TrimSpace(d.Comment)
	if len([]rune(comment)) > 1000 {
		return leadApplication{}, false, errors.New("комментарий длиннее 1000 символов")
	}
	locations := l.locations()
	seen := map[string]bool{}
	for _, p := range locations {
		seen[p.ID] = true
	}
	added := []leadLocation{}
	for _, p := range d.Locations {
		if !catalog.ValidID(p.ID) || strings.TrimSpace(p.Name) == "" {
			return leadApplication{}, false, errors.New("неверная локация")
		}
		if !seen[p.ID] {
			added = append(added, p)
			locations = append(locations, p)
			seen[p.ID] = true
		}
	}
	if len(locations) > 20 {
		return leadApplication{}, false, errors.New("в одном обращении можно обсудить до 20 локаций")
	}
	if len(added) == 0 && comment == "" {
		return leadApplication{}, false, errors.New("добавьте локацию или напишите уточнение")
	}
	next := s.cloneLocked()
	l = next.Submitted[key]
	// Old assignments must not generate a historical taken-in-work notification.
	if l.ServiceVersion == 0 && l.TakenInWork {
		setNotice(&l, "client:taken")
	}
	l.ServiceVersion = max(l.ServiceVersion, 1)
	l.Locations = locations
	l.Updates = append(l.Updates, leadUpdate{ID: len(l.Updates) + 1, Token: token, Locations: added, Comment: comment, CreatedAt: time.Now().UTC()})
	next.Submitted[key] = l
	delete(next.Supplements, key)
	if err := s.saveLocked(next); err != nil {
		return leadApplication{}, false, err
	}
	return cloneLead(l), true, nil
}

// All per-request transitions work for current and historical records.
func (s *leadStore) mutateCase(number int64, fn func(*leadApplication) (bool, error)) (leadApplication, bool, error) {
	if s == nil || number <= 0 {
		return leadApplication{}, false, errors.New("обращение недоступно")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cloneLocked()
	apply := func(l leadApplication, save func(leadApplication)) (leadApplication, bool, error) {
		changed, err := fn(&l)
		if err != nil {
			return leadApplication{}, false, err
		}
		if !changed {
			return cloneLead(l), false, nil
		}
		save(l)
		if l.isResolved() {
			key := strconv.FormatInt(l.UserID, 10)
			if d, ok := next.Supplements[key]; ok && d.Number == l.Number {
				delete(next.Supplements, key)
			}
		}
		if err := s.saveLocked(next); err != nil {
			return leadApplication{}, false, err
		}
		return cloneLead(l), true, nil
	}
	for k, l := range next.Submitted {
		if l.Number == number {
			return apply(l, func(v leadApplication) { next.Submitted[k] = v })
		}
	}
	for i, l := range next.History {
		if l.Number == number {
			return apply(l, func(v leadApplication) { next.History[i] = v })
		}
	}
	return leadApplication{}, false, errors.New("обращение не найдено")
}
func (s *leadStore) acceptUpdate(number int64, updateID int, adminID int64) (leadApplication, bool, error) {
	if adminID <= 0 {
		return leadApplication{}, false, errors.New("неверный специалист")
	}
	return s.mutateCase(number, func(l *leadApplication) (bool, error) {
		if updateID <= 0 || updateID > len(l.Updates) {
			return false, errors.New("дополнение не найдено")
		}
		u := &l.Updates[updateID-1]
		if u.ID != updateID {
			return false, errors.New("неверный номер дополнения")
		}
		if u.AcceptedAt != nil {
			return false, nil
		}
		now := time.Now().UTC()
		if l.isResolved() || l.autoCloseDue(now) {
			return false, errors.New("обращение уже завершено")
		}
		u.AcceptedAt = &now
		u.AcceptedBy = adminID
		return true, nil
	})
}
func (s *leadStore) resolveCase(number, adminID int64) (leadApplication, bool, error) {
	if adminID <= 0 {
		return leadApplication{}, false, errors.New("неверный специалист")
	}
	return s.mutateCase(number, func(l *leadApplication) (bool, error) {
		if l.isResolved() {
			return false, nil
		}
		if !l.TakenInWork {
			return false, errors.New("Сначала возьмите заявку в работу.")
		}
		now := time.Now().UTC()
		if deadline := l.autoCloseAt(); !deadline.IsZero() && !now.Before(deadline) {
			l.complete(deadline, 0)
			return true, nil
		}
		for _, u := range l.Updates {
			if u.AcceptedAt == nil {
				return false, errors.New("Сначала примите все дополнения клиента.")
			}
		}
		l.complete(now, adminID)
		return true, nil
	})
}

func (l *leadApplication) complete(at time.Time, adminID int64) {
	l.ResolvedAt, l.ResolvedBy = &at, adminID
	if l.ServiceVersion == 0 {
		setNotice(l, "client:taken")
	}
	l.ServiceVersion = 2
}

func (l leadApplication) autoCloseAt() time.Time {
	if !l.TakenInWork || l.Hidden || l.isResolved() || l.TakenAt == nil || l.TakenAt.IsZero() {
		return time.Time{}
	}
	return l.TakenAt.Add(time.Hour)
}
func (l leadApplication) autoCloseDue(now time.Time) bool {
	deadline := l.autoCloseAt()
	return !deadline.IsZero() && !now.Before(deadline)
}

func setNotice(l *leadApplication, key string) {
	if l.NoticeReceipts == nil {
		l.NoticeReceipts = map[string]bool{}
	}
	l.NoticeReceipts[key] = true
}
func (s *leadStore) markCaseNotice(number int64, key string, messageID int) error {
	_, _, err := s.mutateCase(number, func(l *leadApplication) (bool, error) {
		if l.NoticeReceipts[key] {
			return false, nil
		}
		setNotice(l, key)
		if messageID > 0 {
			if l.NoticeMessages == nil {
				l.NoticeMessages = map[string]int{}
			}
			l.NoticeMessages[key] = messageID
		}
		return true, nil
	})
	return err
}
func updateKey(id int, stage string, adminID int64) string {
	return fmt.Sprintf("update:%d:%s:%d", id, stage, adminID)
}
