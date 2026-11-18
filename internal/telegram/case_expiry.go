package telegram

import (
	"context"
	"log"
	"time"
)

// Persist all due completions before scheduling their notifications. Archives
// from older releases keep their original status unless newly taken in work.
func (s *leadStore) expireCases(now time.Time) ([]int64, time.Time, error) {
	if s == nil {
		return nil, time.Time{}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var next leadState
	var closed []int64
	var earliest time.Time
	closeIfDue := func(l leadApplication) (leadApplication, bool) {
		deadline := l.autoCloseAt()
		if deadline.IsZero() {
			return l, false
		}
		if now.Before(deadline) {
			if earliest.IsZero() || deadline.Before(earliest) {
				earliest = deadline
			}
			return l, false
		}
		if len(closed) == 0 {
			next = s.cloneLocked()
		}
		l = cloneLead(l)
		l.complete(deadline, 0)
		closed = append(closed, l.Number)
		return l, true
	}
	for key, l := range s.state.Submitted {
		if updated, ok := closeIfDue(l); ok {
			next.Submitted[key] = updated
			if d, exists := next.Supplements[key]; exists && d.Number == l.Number {
				delete(next.Supplements, key)
			}
		}
	}
	for i, l := range s.state.History {
		if l.ServiceVersion >= 2 {
			if updated, ok := closeIfDue(l); ok {
				next.History[i] = updated
			}
		}
	}
	if len(closed) > 0 {
		if err := s.saveLocked(next); err != nil {
			return nil, time.Time{}, err
		}
	}
	return closed, earliest, nil
}

func (b *Bot) expireDueCases() time.Time {
	closed, earliest, err := b.leads.expireCases(time.Now().UTC())
	if err != nil {
		log.Printf("Auto-complete applications: %v", err)
	}
	for _, number := range closed {
		b.queueLeadDelivery(number)
	}
	return earliest
}

// Known deadlines have a precise wakeup. Discover new assignments and
// restored data within a minute, independently of Telegram delivery.
func (b *Bot) runCaseExpiry(ctx context.Context, earliest time.Time) {
	for ctx.Err() == nil {
		wait := time.Minute
		if !earliest.IsZero() {
			wait = min(wait, time.Until(earliest))
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			earliest = b.expireDueCases()
		}
	}
}
