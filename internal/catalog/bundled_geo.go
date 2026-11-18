package catalog

import (
	_ "embed"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Bundled positions are the 104 unambiguous matches from the owner's
// Yandex Constructor export (2026-11-16). Do not infer positions for the
// other 10 locations until their names have been confirmed.

//go:embed bundled_geo_104.csv
var verifiedGeoCSV string

func bundledGeoOverrides() (map[string]Override, error) {
	locations := make(map[string]Override, 104)
	for number, line := range strings.Split(strings.TrimSpace(verifiedGeoCSV), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		columns := strings.Split(line, ",")
		if len(columns) != 3 {
			return nil, fmt.Errorf("curated geo row %d: expected ID,lat,lon", number+1)
		}
		id, latText, lonText := strings.TrimSpace(columns[0]), strings.TrimSpace(columns[1]), strings.TrimSpace(columns[2])
		if !validID(id) {
			return nil, fmt.Errorf("curated geo row %d: invalid ID", number+1)
		}
		if _, duplicate := locations[id]; duplicate {
			return nil, fmt.Errorf("curated geo row %d: duplicate ID %s", number+1, id)
		}
		lat, latErr := strconv.ParseFloat(latText, 64)
		lon, lonErr := strconv.ParseFloat(lonText, 64)
		if latErr != nil || lonErr != nil || math.IsNaN(lat) || math.IsNaN(lon) ||
			math.IsInf(lat, 0) || math.IsInf(lon, 0) || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			return nil, fmt.Errorf("curated geo row %d: invalid latitude or longitude", number+1)
		}
		// Direct single-point Yandex link (not a view of the entire layer).
		// Match the verified ll/pt/z format in the owner's exported CSV.
		pointURL := fmt.Sprintf("https://yandex.ru/maps/?ll=%s%%2C%s&z=17&pt=%s%%2C%s,pm2rdm", lonText, latText, lonText, latText)
		locations[id] = Override{Latitude: &lat, Longitude: &lon, MapURL: &pointURL}
	}
	if len(locations) != 104 {
		return nil, fmt.Errorf("curated geo: expected 104 confirmed coordinates, got %d", len(locations))
	}
	return locations, nil
}
