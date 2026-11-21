package telegram

import (
 "maps"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

type leadDraft struct {
	Source      string `json:"source"`
	SuggestedID string `json:"suggested_id,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Company     string `json:"company,omitempty"`
	Name        string `json:"name,omitempty"`
	Phone       string `json:"phone,omitempty"`
	LocationID  string `json:"location_id,omitempty"`
	Stage       string `json:"stage"`
	Page        int    `json:"page,omitempty"`
	Paused      bool   `json:"paused,omitempty"`
}

type leadApplication struct {
	Number         int64           `json:"number"`
	UserID         int64           `json:"user_id"`
	Username       string          `json:"username,omitempty"`
	Kind           string          `json:"kind"`
	Company        string          `json:"company,omitempty"`
	Name           string          `json:"name"`
	Phone          string          `json:"phone"`
	LocationID     string          `json:"location_id"`
	LocationName   string          `json:"location_name"`
	Source         string          `json:"source"`
	CreatedAt      time.Time       `json:"created_at"`
	Delivered      map[string]bool `json:"delivered,omitempty"`
	AdminMessages  map[string]int  `json:"admin_messages,omitempty"`
	TakenInWork    bool            `json:"taken_in_work,omitempty"`
	TakenBy        int64           `json:"taken_by,omitempty"`
	TakenAt        *time.Time      `json:"taken_at,omitempty"`
	Hidden         bool            `json:"hidden,omitempty"`
	Locations      []leadLocation  `json:"locations,omitempty"`
	Updates        []leadUpdate    `json:"updates,omitempty"`
	ResolvedAt     *time.Time      `json:"resolved_at,omitempty"`
	ResolvedBy     int64           `json:"resolved_by,omitempty"`
	Resolution     string          `json:"resolution,omitempty"`
	ServiceVersion int             `json:"service_version,omitempty"`
	ReferralCode string `json:"referral_code,omitempty"`
	ReferrerID int64 `json:"referrer_id,omitempty"`
	ReferrerName string `json:"referrer_name,omitempty"`
	ReferralStatus string `json:"referral_status,omitempty"`
	NoticeReceipts map[string]bool `json:"notice_receipts,omitempty"`
	NoticeMessages map[string]int  `json:"notice_messages,omitempty"`
}

type leadState struct {
	Drafts      map[string]leadDraft       `json:"drafts"`
	Submitted   map[string]leadApplication `json:"submitted"`
	History     []leadApplication          `json:"history,omitempty"`
	NextID      int64                      `json:"next_id"`
	Supplements map[string]supplementDraft `json:"supplements,omitempty"`
	Profiles     map[string]miniProfileRecord `json:"profiles,omitempty"`
	ReferralCodes map[string]string `json:"referral_codes,omitempty"`
	PendingReferrals map[string]string `json:"pending_referrals,omitempty"`
	Referrals map[string]referralRecord `json:"referrals,omitempty"`
 DiscountUses map[int64]discountUseRecord `json:"discount_uses,omitempty"`
}

type leadStore struct {
	mu    sync.Mutex
	path  string
	state leadState
}

func openLeadStore(path string) (*leadStore, error) {
	s := &leadStore{path: path, state: leadState{Drafts: map[string]leadDraft{}, Submitted: map[string]leadApplication{}, Profiles: map[string]miniProfileRecord{}, ReferralCodes:map[string]string{}, PendingReferrals:map[string]string{}, Referrals:map[string]referralRecord{}}}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read lead store: %w", err)
	}
	if err := json.Unmarshal(raw, &s.state); err != nil {
		return nil, fmt.Errorf("decode lead store: %w", err)
	}
	if s.state.Drafts == nil {
		s.state.Drafts = map[string]leadDraft{}
	}
	if s.state.Submitted == nil {
		s.state.Submitted = map[string]leadApplication{}
	}
	if s.state.Profiles == nil {
		s.state.Profiles = map[string]miniProfileRecord{}
	}
	if s.state.ReferralCodes == nil {s.state.ReferralCodes=map[string]string{}}
	if s.state.PendingReferrals == nil {s.state.PendingReferrals=map[string]string{}}
	if s.state.Referrals == nil {s.state.Referrals=map[string]referralRecord{}}
	// Resume stored forms explicitly after a restart.
	for k, d := range s.state.Drafts {
		d.Paused = true
		s.state.Drafts[k] = d
	}
	for k, d := range s.state.Supplements {
		d.Paused = true
		s.state.Supplements[k] = d
	}
	if s.state.History == nil {
		s.state.History = []leadApplication{}
	}
	return s, nil
}

func (s *leadStore) draft(userID int64) (leadDraft, bool) {
	if s == nil || userID <= 0 {
		return leadDraft{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.state.Drafts[strconv.FormatInt(userID, 10)]
	return cloneLeadDraft(d), ok
}

func (s *leadStore) submitted(userID int64) (leadApplication, bool) {
	if s == nil || userID <= 0 {
		return leadApplication{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.state.Submitted[strconv.FormatInt(userID, 10)]
	return cloneLead(l), ok
}

func cloneLead(lead leadApplication) leadApplication {
	copyLead := lead
	cloneServiceFields(&copyLead)
	if lead.Delivered != nil {
		copyLead.Delivered = make(map[string]bool, len(lead.Delivered))
		for id, sent := range lead.Delivered {
			copyLead.Delivered[id] = sent
		}
	}
	if lead.AdminMessages != nil {
		copyLead.AdminMessages = make(map[string]int, len(lead.AdminMessages))
		for id, messageID := range lead.AdminMessages {
			copyLead.AdminMessages[id] = messageID
		}
	}
	if lead.TakenAt != nil {
		taken := *lead.TakenAt
		copyLead.TakenAt = &taken
	}
	return copyLead
}

func (s *leadStore) cloneLocked() leadState {
	next := leadState{
		Drafts:      make(map[string]leadDraft, len(s.state.Drafts)),
		Submitted:   make(map[string]leadApplication, len(s.state.Submitted)),
		History:     make([]leadApplication, 0, len(s.state.History)),
		NextID:      s.state.NextID,
		Supplements: make(map[string]supplementDraft, len(s.state.Supplements)),
		Profiles: make(map[string]miniProfileRecord, len(s.state.Profiles)),
		ReferralCodes:make(map[string]string,len(s.state.ReferralCodes)),
		PendingReferrals:make(map[string]string,len(s.state.PendingReferrals)),
		Referrals:make(map[string]referralRecord,len(s.state.Referrals)),
	}
	for key, draft := range s.state.Drafts {
		next.Drafts[key] = cloneLeadDraft(draft)
	}
	for key, profile := range s.state.Profiles {
		next.Profiles[key] = profile
	}
	for k,v:=range s.state.ReferralCodes {next.ReferralCodes[k]=v}
	for k,v:=range s.state.PendingReferrals {next.PendingReferrals[k]=v}
	for k,v:=range s.state.Referrals {next.Referrals[k]=v}
 next.DiscountUses=make(map[int64]discountUseRecord,len(s.state.DiscountUses))
 for key,rec:=range s.state.DiscountUses {rec.AdminMessages=maps.Clone(rec.AdminMessages);next.DiscountUses[key]=rec}
	for key, draft := range s.state.Supplements {
		next.Supplements[key] = cloneSupplementDraft(draft)
	}
	for key, lead := range s.state.Submitted {
		next.Submitted[key] = cloneLead(lead)
	}
	for _, lead := range s.state.History {
		next.History = append(next.History, cloneLead(lead))
	}
	return next
}

func (s *leadStore) saveLocked(next leadState) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	content, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".lead-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(content)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		return err
	}
	s.state = next
	return nil
}

func (s *leadStore) putDraft(userID int64, d leadDraft) error {
	if s == nil || userID <= 0 {
		return errors.New("invalid lead draft")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := strconv.FormatInt(userID, 10)
	if previous, sent := s.state.Submitted[key]; sent && !previous.isResolved() {
		return errors.New("lead already submitted")
	}
	next := s.cloneLocked()
	next.Drafts[key] = d
	return s.saveLocked(next)
}

func (s *leadStore) clearDraft(userID int64) error {
	if s == nil || userID <= 0 {
		return errors.New("invalid user")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cloneLocked()
	delete(next.Drafts, strconv.FormatInt(userID, 10))
	return s.saveLocked(next)
}

// submit is atomic under the store lock: two simultaneous clicks cannot
// create two applications, and the draft disappears only after disk commit.
func (s *leadStore) submit(userID int64, username, locationName string) (leadApplication, bool, error) {
	if s == nil || userID <= 0 {
		return leadApplication{}, false, errors.New("invalid user")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.state.Drafts[strconv.FormatInt(userID, 10)]
	return s.submitLocked(userID, username, locationName, d, "", false)
}

// submitMiniCase commits exactly the contact data and referral choice from one request.
func (s *leadStore) submitMiniCase(userID int64, username, locationName string, d leadDraft, referralCode string, skipReferral bool) (leadApplication, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.submitLocked(userID, username, locationName, d, referralCode, skipReferral)
}

func (s *leadStore) submitLocked(userID int64, username, locationName string, d leadDraft, referralCode string, skipReferral bool) (leadApplication, bool, error) {
	key := strconv.FormatInt(userID, 10)
	if previous, exists := s.state.Submitted[key]; exists && !previous.isResolved() {
		return cloneLead(previous), true, nil
	}
	if d.Kind == "" || d.Name == "" || d.Phone == "" || d.LocationID == "" || (d.Kind == "new" && d.Company == "") {
		return leadApplication{}, false, errors.New("incomplete application")
	}
	code := s.state.PendingReferrals[key]
	if skipReferral {
		code = ""
	} else if referralCode != "" {
		validated, err := s.validateReferralLocked(userID, referralCode)
		if err != nil {
			return leadApplication{}, false, err
		}
		code = validated
	}
	next := s.cloneLocked()
	if previous, exists := next.Submitted[key]; exists {
		// Keep the previous processed application in the permanent history
		// instead of overwriting the only record for this Telegram user.
		if !previous.isResolved() {
			return cloneLead(previous), true, nil
		}
		next.History = append(next.History, cloneLead(previous))
	}
	next.NextID++
	lead := leadApplication{
		Number: next.NextID, UserID: userID, Username: username,
		Kind: d.Kind, Company: d.Company, Name: d.Name, Phone: d.Phone,
		LocationID: d.LocationID, LocationName: locationName, Source: d.Source,
		CreatedAt: time.Now().UTC(), Delivered: map[string]bool{}, ServiceVersion: 1,
		Locations: []leadLocation{{ID: d.LocationID, Name: locationName}},
	}
	// Record the first verified invitation atomically with the submitted case.
	if code != "" && !s.alreadyAppliedLocked(userID) {
		owner := s.referralOwnerLocked(code)
		if owner > 0 && owner != userID {
			inviterName := s.referralDisplayNameLocked(owner)
			lead.ReferralCode, lead.ReferrerID = code, owner
			lead.ReferrerName, lead.ReferralStatus = inviterName, "pending"
			next.Referrals[key] = referralRecord{
				InviteeID: userID, InviterID: owner, CaseNumber: lead.Number,
				InviterName: inviterName, InviteeName: lead.Name, Code: code,
				Status: "pending", CreatedAt: lead.CreatedAt,
			}
		}
	}
	delete(next.PendingReferrals,key)
	next.Submitted[key] = lead
	delete(next.Drafts, key)
	delete(next.Supplements, key)
	if err := s.saveLocked(next); err != nil {
		return leadApplication{}, false, err
	}
	return lead, false, nil
}

// markDelivered stores the Telegram message ID with the application number.
// The number matters: one user can now submit multiple processed applications.
func (s *leadStore) markDelivered(userID, number, adminID int64, messageID int) error {
	if s == nil || userID <= 0 || number <= 0 || adminID <= 0 {
		return errors.New("invalid application delivery")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cloneLocked()
	modify := func(lead leadApplication) leadApplication {
		if lead.Delivered == nil {
			lead.Delivered = map[string]bool{}
		}
		key := strconv.FormatInt(adminID, 10)
		lead.Delivered[key] = true
		if messageID > 0 {
			if lead.AdminMessages == nil {
				lead.AdminMessages = map[string]int{}
			}
			lead.AdminMessages[key] = messageID
		}
		return lead
	}
	key := strconv.FormatInt(userID, 10)
	if lead, ok := next.Submitted[key]; ok && lead.Number == number {
		next.Submitted[key] = modify(lead)
		return s.saveLocked(next)
	}
	for i := range next.History {
		if next.History[i].Number == number && next.History[i].UserID == userID {
			next.History[i] = modify(next.History[i])
			return s.saveLocked(next)
		}
	}
	return errors.New("application not found for delivery receipt")
}

func (s *leadStore) byNumber(number int64) (leadApplication, bool) {
	if s == nil || number <= 0 {
		return leadApplication{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, lead := range s.state.Submitted {
		if lead.Number == number {
			return cloneLead(lead), true
		}
	}
	for _, lead := range s.state.History {
		if lead.Number == number {
			return cloneLead(lead), true
		}
	}
	return leadApplication{}, false
}

// takeInWork assigns the first administrator atomically. The application
// keeps its original TakenAt on subsequent clicks and after a restart.
func (s *leadStore) takeInWork(number, adminID int64) (leadApplication, bool, error) {
	if s == nil || number <= 0 || adminID <= 0 {
		return leadApplication{}, false, errors.New("invalid application or administrator")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cloneLocked()
	now := time.Now().UTC()
	for key, lead := range next.Submitted {
		if lead.Number != number {
			continue
		}
		if lead.isResolved() {
			return cloneLead(lead), false, errors.New("обращение уже решено")
		}
		if lead.TakenInWork {
			return cloneLead(lead), false, nil
		}
		lead.TakenInWork, lead.TakenBy, lead.TakenAt = true, adminID, &now
		lead.ServiceVersion = 2
		next.Submitted[key] = lead
		if err := s.saveLocked(next); err != nil {
			return leadApplication{}, false, err
		}
		return cloneLead(lead), true, nil
	}
	for i, lead := range next.History {
		if lead.Number != number {
			continue
		}
		if lead.isResolved() {
			return cloneLead(lead), false, errors.New("обращение уже решено")
		}
		if lead.TakenInWork {
			return cloneLead(lead), false, nil
		}
		lead.TakenInWork, lead.TakenBy, lead.TakenAt = true, adminID, &now
		lead.ServiceVersion = 2
		next.History[i] = lead
		if err := s.saveLocked(next); err != nil {
			return leadApplication{}, false, err
		}
		return cloneLead(lead), true, nil
	}
	return leadApplication{}, false, errors.New("application number not found")
}

func (s *leadStore) allSubmitted() []leadApplication {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]leadApplication, 0, len(s.state.Submitted)+len(s.state.History))
	for _, lead := range s.state.Submitted {
		out = append(out, cloneLead(lead))
	}
	for _, lead := range s.state.History {
		out = append(out, cloneLead(lead))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number > out[j].Number })
	return out
}

// visibleSubmitted is for UI and analytics. Hidden records are retained in
// the durable store so a soft deletion can be undone without changing IDs.
func (s *leadStore) visibleSubmitted() []leadApplication {
	all := s.allSubmitted()
	visible := make([]leadApplication, 0, len(all))
	for _, lead := range all {
		if !lead.Hidden {
			visible = append(visible, lead)
		}
	}
	return visible
}
func (s *leadStore) setHidden(numbers []int64, hidden bool) error {
	if s == nil {
		return errors.New("lead store unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cloneLocked()
	seen := map[int64]bool{}
	for _, n := range numbers {
		if n <= 0 || seen[n] {
			return errors.New("invalid or duplicate application number")
		}
		seen[n] = true
		found := false
		for id, lead := range next.Submitted {
			if lead.Number != n {
				continue
			}
			if hidden && !lead.isResolved() {
				return fmt.Errorf("сначала завершите обращение №%04d", n)
			}
			if lead.Hidden == hidden {
				return fmt.Errorf("заявка №%04d уже имеет этот статус", n)
			}
			lead.Hidden = hidden
			next.Submitted[id] = lead
			found = true
			break
		}
		if !found {
			for i, lead := range next.History {
				if lead.Number != n {
					continue
				}
				if hidden && !lead.isResolved() {
					return fmt.Errorf("сначала завершите обращение №%04d", n)
				}
				if lead.Hidden == hidden {
					return fmt.Errorf("заявка №%04d уже имеет этот статус", n)
				}
				lead.Hidden = hidden
				next.History[i] = lead
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("заявка №%04d не найдена", n)
		}
	}
	return s.saveLocked(next)
}

// eraseHidden removes only previously hidden and processed applications,
// preserving the globally monotonic NextID for future user submissions.
func (s *leadStore) eraseHidden(numbers []int64) error {
	if s == nil {
		return errors.New("lead store unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cloneLocked()
	wanted := map[int64]bool{}
	for _, n := range numbers {
		if n <= 0 || wanted[n] {
			return errors.New("invalid or duplicate application number")
		}
		wanted[n] = true
	}
	for _, n := range numbers {
		found := false
		for user, lead := range next.Submitted {
			if lead.Number != n {
				continue
			}
			if !lead.Hidden || !lead.isResolved() {
				return fmt.Errorf("заявка №%04d не находится в корзине", n)
			}
			delete(next.Submitted, user)
			found = true
			break
		}
		if !found {
			for i, lead := range next.History {
				if lead.Number != n {
					continue
				}
				if !lead.Hidden || !lead.isResolved() {
					return fmt.Errorf("заявка №%04d не находится в корзине", n)
				}
				next.History = append(next.History[:i], next.History[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("заявка №%04d не найдена", n)
		}
	}
	return s.saveLocked(next)
}
