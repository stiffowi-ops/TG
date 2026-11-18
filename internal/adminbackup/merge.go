package adminbackup

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"locationbot/internal/analytics"
	"locationbot/internal/catalog"
	"sort"
	"strings"
	"time"
)

type leadSnapshot struct {
	Drafts      map[string]json.RawMessage `json:"drafts"`
	Submitted   map[string]json.RawMessage `json:"submitted"`
	History     []json.RawMessage          `json:"history,omitempty"`
	NextID      int64                      `json:"next_id"`
	Supplements map[string]json.RawMessage `json:"supplements,omitempty"`
}

type leadIdentity struct {
	Number      int64 `json:"number"`
	UserID      int64 `json:"user_id"`
	TakenInWork bool  `json:"taken_in_work"`
}

func decodeLead(raw json.RawMessage) (leadIdentity, error) {
	var l leadIdentity
	err := json.Unmarshal(raw, &l)
	if err != nil {
		return l, err
	}
	if l.Number <= 0 || l.UserID < 0 {
		return l, errors.New("invalid application identity")
	}
	return l, nil
}
func parseLeads(raw []byte) (leadSnapshot, error) {
	var result leadSnapshot
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte(`{"drafts":{},"submitted":{},"next_id":0}`)
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return result, err
	}
	if result.Drafts == nil {
		result.Drafts = map[string]json.RawMessage{}
	}
	if result.Submitted == nil {
		result.Submitted = map[string]json.RawMessage{}
	}
	if result.History == nil {
		result.History = []json.RawMessage{}
	}
	if result.NextID < 0 {
		return result, errors.New("negative application counter")
	}
	seen := map[int64]bool{}
	check := func(raw json.RawMessage, expectedUser string) error {
		d, err := decodeLead(raw)
		if err != nil {
			return err
		}
		if expectedUser != "" && d.UserID > 0 && expectedUser != fmt.Sprint(d.UserID) {
			return errors.New("application user key mismatch")
		}
		if expectedUser == "" && d.UserID <= 0 {
			return errors.New("history application missing user ID")
		}
		if d.Number > result.NextID {
			return errors.New("application counter is lower than stored number")
		}
		if seen[d.Number] {
			return fmt.Errorf("duplicate application number %d", d.Number)
		}
		seen[d.Number] = true
		return nil
	}
	for user, lead := range result.Submitted {
		if !digitsOnly(user) {
			return result, fmt.Errorf("invalid user ID %s", user)
		}
		if err := check(lead, user); err != nil {
			return result, err
		}
	}
	for _, lead := range result.History {
		if err := check(lead, ""); err != nil {
			return result, err
		}
	}
	for user := range result.Drafts {
		if !digitsOnly(user) {
			return result, fmt.Errorf("invalid draft user ID %s", user)
		}
	}
	return result, nil
}
func digitsOnly(value string) bool {
	if value == "" {
		return false
	}
	for _, c := range value {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Merge archived and current applications without resurrecting an old pending
// request after it has already been taken into work. Current state wins for a
// given application; all distinct numbers survive in the permanent history.
func mergeLeads(archived, current []byte) ([]byte, error) {
	old, err := parseLeads(archived)
	if err != nil {
		return nil, err
	}
	now, err := parseLeads(current)
	if err != nil {
		return nil, err
	}
	out := leadSnapshot{
		Drafts:    map[string]json.RawMessage{},
		Submitted: map[string]json.RawMessage{},
		History:   []json.RawMessage{},
		NextID:    max(old.NextID, now.NextID),
	}
	for k, v := range old.Drafts {
		out.Drafts[k] = v
	}
	for k, v := range now.Drafts {
		out.Drafts[k] = v
	}
	used := map[int64]int64{}
	addCurrent := func(raw json.RawMessage, userKey string, historical bool) error {
		lead, e := decodeLead(raw)
		if e != nil {
			return e
		}
		if lead.UserID == 0 && userKey != "" {
			var id int64
			id, e = json.Number(userKey).Int64()
			if e != nil {
				return e
			}
			lead.UserID = id
		}
		if lead.UserID <= 0 {
			return errors.New("application missing user ID")
		}
		if prior, exists := used[lead.Number]; exists && prior != lead.UserID {
			return errors.New("duplicate current application number")
		}
		used[lead.Number] = lead.UserID
		if historical {
			out.History = append(out.History, raw)
		} else {
			out.Submitted[userKey] = raw
		}
		return nil
	}
	for _, raw := range now.History {
		if e := addCurrent(raw, "", true); e != nil {
			return nil, e
		}
	}
	for user, raw := range now.Submitted {
		if e := addCurrent(raw, user, false); e != nil {
			return nil, e
		}
	}
	// The identity of an application is its global number and user ID.
	// Do not restore an archived pending copy if current history already
	// contains the same (now processed) application.
	addArchive := func(raw json.RawMessage, userKey string, historical bool) error {
		lead, e := decodeLead(raw)
		if e != nil {
			return e
		}
		if lead.UserID == 0 && userKey != "" {
			lead.UserID, e = json.Number(userKey).Int64()
			if e != nil {
				return e
			}
		}
		if lead.UserID <= 0 {
			return errors.New("archived application missing user ID")
		}
		originalNumber := lead.Number
		if prior, exists := used[lead.Number]; exists {
			if prior == lead.UserID {
				return nil
			}
			// Collision between two distinct legacy copies: retain the
			// current number and assign a new globally unique archive ID.
			out.NextID++
			lead.Number = out.NextID
		}
		if userKey == "" {
			userKey = fmt.Sprint(lead.UserID)
		}
		if !historical {
			if _, present := out.Submitted[userKey]; present {
				historical = true
			}
		}
		if lead.Number != originalNumber || lead.UserID != 0 {
			var fields map[string]json.RawMessage
			if e := json.Unmarshal(raw, &fields); e != nil {
				return e
			}
			fields["number"], _ = json.Marshal(lead.Number)
			fields["user_id"], _ = json.Marshal(lead.UserID)
			raw, e = json.Marshal(fields)
			if e != nil {
				return e
			}
		}
		used[lead.Number] = lead.UserID
		if historical {
			out.History = append(out.History, raw)
		} else {
			out.Submitted[userKey] = raw
		}
		return nil
	}
	for _, raw := range old.History {
		if e := addArchive(raw, "", true); e != nil {
			return nil, e
		}
	}
	for user, raw := range old.Submitted {
		if e := addArchive(raw, user, false); e != nil {
			return nil, e
		}
	}
	out.Supplements = map[string]json.RawMessage{}
	for user, draft := range old.Supplements {
		out.Supplements[user] = draft
	}
	for user, draft := range now.Supplements {
		out.Supplements[user] = draft
	}
	// Keep a draft only for its current open request. A submitted token must
	// never be restored as an unsent supplement from an older ZIP.
	for user, raw := range out.Supplements {
		var d struct {
			Number int64  `json:"number"`
			Token  string `json:"token"`
		}
		var l struct {
			Number     int64      `json:"number"`
			ResolvedAt *time.Time `json:"resolved_at"`
			Updates    []struct {
				Token string `json:"token"`
			} `json:"updates"`
		}
		if json.Unmarshal(raw, &d) != nil || json.Unmarshal(out.Submitted[user], &l) != nil || d.Token == "" || d.Number != l.Number || l.ResolvedAt != nil {
			delete(out.Supplements, user)
			continue
		}
		for _, u := range l.Updates {
			if u.Token == d.Token {
				delete(out.Supplements, user)
				break
			}
		}
	}
	for user, raw := range out.Submitted {
		var lead leadIdentity
		if e := json.Unmarshal(raw, &lead); e != nil {
			return nil, e
		}
		var state struct {
			ResolvedAt *time.Time `json:"resolved_at"`
		}
		if e := json.Unmarshal(raw, &state); e != nil {
			return nil, e
		}
		if state.ResolvedAt == nil {
			delete(out.Drafts, user)
		}
	}
	sort.Slice(out.History, func(i, j int) bool {
		var a, b leadIdentity
		_ = json.Unmarshal(out.History[i], &a)
		_ = json.Unmarshal(out.History[j], &b)
		return a.Number < b.Number
	})
	return json.MarshalIndent(out, "", "  ")
}

func mergeAnalytics(archived, current []byte) ([]byte, error) {
	events := map[string]struct{}{}
	for _, data := range [][]byte{archived, current} {
		scanner := bufio.NewScanner(bytes.NewReader(data))
		scanner.Buffer(make([]byte, 4096), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var record struct {
				At         string `json:"at"`
				Kind       string `json:"kind"`
				UserID     int64  `json:"user_id"`
				LocationID string `json:"location_id"`
			}
			if err := json.Unmarshal([]byte(line), &record); err != nil {
				return nil, err
			}
			timestamp, err := time.Parse(time.RFC3339Nano, record.At)
			if err != nil {
				return nil, err
			}
			if err = (analytics.Event{At: timestamp, UserID: record.UserID, LocationID: record.LocationID, Kind: record.Kind}).Validate(); err != nil {
				return nil, err
			}
			events[line] = struct{}{}
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
	}
	lines := make([]string, 0, len(events))
	for line := range events {
		lines = append(lines, line)
	}
	sort.Strings(lines)
	if len(lines) == 0 {
		return []byte{}, nil
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

// mergeFavorites protects the current users' choices when an older archive is
// restored. Archive-only users are kept unless a post-backup removal cleared
// their last favorite, which is visible in the current analytics event log.
func mergeFavorites(archived, current, events []byte, archivedAt time.Time) ([]byte, error) {
	read := func(raw []byte) (map[string][]string, error) {
		if len(bytes.TrimSpace(raw)) == 0 {
			return map[string][]string{}, nil
		}
		var data map[string][]string
		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, err
		}
		if data == nil {
			data = map[string][]string{}
		}
		for user, ids := range data {
			if !digitsOnly(user) {
				return nil, fmt.Errorf("invalid favorite user ID %q", user)
			}
			seen := map[string]bool{}
			for _, id := range ids {
				if !catalog.ValidID(id) || seen[id] {
					return nil, fmt.Errorf("invalid favorite location %q", id)
				}
				seen[id] = true
			}
		}
		return data, nil
	}
	old, err := read(archived)
	if err != nil {
		return nil, err
	}
	now, err := read(current)
	if err != nil {
		return nil, err
	}
	for user, ids := range now {
		old[user] = append([]string(nil), ids...)
	}
	scanner := bufio.NewScanner(bytes.NewReader(events))
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		var e analytics.Event
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			return nil, err
		}
		if err := e.Validate(); err != nil {
			return nil, err
		}
		if e.At.Before(archivedAt) || (e.Kind != analytics.FavoriteAdded && e.Kind != analytics.FavoriteRemoved) {
			continue
		}
		user := fmt.Sprint(e.UserID)
		if _, present := now[user]; !present {
			// Current empty state is represented by absence of a user key.
			delete(old, user)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return json.MarshalIndent(old, "", "  ")
}

// mergeRevisionHistory keeps newer admin audit entries when restoring an older
// archive. Every record is deduplicated by canonical JSON and sorted by time.
func mergeRevisionHistory(archived, current []byte) ([]byte, error) {
	seen := map[string]json.RawMessage{}
	for _, raw := range [][]byte{archived, current} {
		if len(bytes.TrimSpace(raw)) == 0 {
			raw = []byte("[]")
		}
		var rows []json.RawMessage
		if err := json.Unmarshal(raw, &rows); err != nil {
			return nil, err
		}
		for _, item := range rows {
			var check struct {
				At         time.Time `json:"at"`
				LocationID string    `json:"location_id"`
			}
			if err := json.Unmarshal(item, &check); err != nil || check.At.IsZero() || !catalog.ValidID(check.LocationID) {
				return nil, errors.New("invalid revision history record")
			}
			var compact bytes.Buffer
			if err := json.Compact(&compact, item); err != nil {
				return nil, err
			}
			seen[compact.String()] = json.RawMessage(compact.String())
		}
	}
	rows := make([]json.RawMessage, 0, len(seen))
	for _, v := range seen {
		rows = append(rows, v)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		var a, b struct {
			At time.Time `json:"at"`
		}
		_ = json.Unmarshal(rows[i], &a)
		_ = json.Unmarshal(rows[j], &b)
		return a.At.Before(b.At)
	})
	return json.MarshalIndent(rows, "", "  ")
}
