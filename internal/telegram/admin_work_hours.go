package telegram

import (
	"strings"
	"time"
)

const adminWorkStartHour = 9
const adminWorkEndHour = 19

// The agreed calendar is Monday-Friday, 09:00 inclusive to 19:00 exclusive.
// A fixed Moscow zone makes it independent of the server's local timezone.
func adminWorkday(at time.Time) bool {
	day := at.In(visitTZ).Weekday()
	return day != time.Saturday && day != time.Sunday
}

func adminWorkTime(at time.Time) bool {
	hour := at.In(visitTZ).Hour()
	return adminWorkday(at) && hour >= adminWorkStartHour && hour < adminWorkEndHour
}

// Count only weekday work windows. Full weeks take constant time; the tail
// checks at most six days, irrespective of the age of the application.
func adminWorkingDuration(start, end time.Time) time.Duration {
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return 0
	}
	start = start.In(visitTZ)
	end = end.In(visitTZ)
	midnight := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, visitTZ) }
	workDay := time.Duration(adminWorkEndHour-adminWorkStartHour) * time.Hour
	withinDay := func(t time.Time) time.Duration {
		if !adminWorkday(t) {
			return 0
		}
		elapsed := t.Sub(midnight(t).Add(adminWorkStartHour * time.Hour))
		if elapsed < 0 {
			return 0
		}
		if elapsed > workDay {
			return workDay
		}
		return elapsed
	}
	startDay := midnight(start)
	days := int(midnight(end).Sub(startDay) / (24 * time.Hour))
	weekdays := (days / 7) * 5
	for day := 0; day < days%7; day++ {
		if adminWorkday(startDay.AddDate(0, 0, day)) {
			weekdays++
		}
	}
	return time.Duration(weekdays)*workDay + withinDay(end) - withinDay(start)
}

func adminReminderStage(created, now time.Time) string {
	if !adminWorkTime(now) {
		return ""
	}
	elapsed := adminWorkingDuration(created, now)
	if elapsed >= supplementEscalationDelay {
		return "escalation"
	}
	if elapsed >= supplementReminderDelay {
		return "reminder"
	}
	return ""
}

func isAdminReminderKey(key string) bool {
	// Preserve the per-event delivery keys already stored in runtime.
	return strings.HasPrefix(key, "case:") || strings.Contains(key, ":reminder:") || strings.Contains(key, ":escalation:")
}
