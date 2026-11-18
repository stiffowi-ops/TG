package telegram

import (
	"regexp"
	"strings"

	"locationbot/internal/catalog"
)

// Keep review notes in the bundled data/overrides for internal verification;
// remove them only from user-visible copies. Never assert that an address has
// been checked just because its review marker is hidden.
var reviewMarker = regexp.MustCompile(`(?i)\s*\(\s*данные\s+проверить\s*\)`)

func publicText(value string) string {
	return strings.TrimSpace(reviewMarker.ReplaceAllString(value, ""))
}

func publicLocation(value catalog.Location) catalog.Location {
	value.Title = publicText(value.Title)
	value.Category = publicText(value.Category)
	value.Description = publicText(value.Description)
	value.Rating = publicText(value.Rating)
	value.Landmarks = publicText(value.Landmarks)
	value.LineStatus = publicText(value.LineStatus)
	value.Traffic = publicText(value.Traffic)
	value.Payers = publicText(value.Payers)
	value.Opportunity = publicText(value.Opportunity)
	value.Conclusion = publicText(value.Conclusion)
	return value
}
