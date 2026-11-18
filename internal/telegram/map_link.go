package telegram

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var httpLinkRx = regexp.MustCompile(`https?://[^\s<>"']+`)
var errMapPoint = errors.New("cannot determine map point")

// sharedMapInput recognizes coordinates and Yandex Maps links without taking
// ordinary location-name searches away from the existing text search.
func sharedMapInput(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if text == "" || len(text) > 3000 {
		return "", false
	}
	if _, ok := parseCopiedCoordinates(text); ok {
		return text, true
	}
	for _, part := range httpLinkRx.FindAllString(text, -1) {
		part = strings.TrimRight(part, ".,;!?)]}»")
		if u, err := url.Parse(part); err == nil && allowedMapURL(u) {
			return part, true
		}
	}
	return "", false
}

func validMapPoint(lat, lon float64) bool {
	return !math.IsNaN(lat) && !math.IsNaN(lon) && !math.IsInf(lat, 0) && !math.IsInf(lon, 0) &&
		lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}

// Yandex place cards copy coordinates as latitude, longitude.
// Yandex Maps URLs contain them as longitude, latitude.
func parseCopiedCoordinates(text string) (*geoPoint, bool) {
	fields := strings.FieldsFunc(strings.TrimSpace(text), func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t'
	})
	if len(fields) != 2 {
		return nil, false
	}
	first, err1 := strconv.ParseFloat(fields[0], 64)
	second, err2 := strconv.ParseFloat(fields[1], 64)
	if err1 != nil || err2 != nil {
		return nil, false
	}
	lat, lon := first, second
	// Only swap longitude-first copied values when they unmistakably
	// represent the Moscow/Central Russia region served by this catalogue.
	if first >= 30 && first <= 45 && second >= 50 && second <= 60 {
		lat, lon = second, first
	}
	if !validMapPoint(lat, lon) {
		return nil, false
	}
	return &geoPoint{Latitude: lat, Longitude: lon}, true
}

func allowedMapURL(u *url.URL) bool {
	if u == nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "yandex.ru", "www.yandex.ru", "m.yandex.ru", "yandex.com", "www.yandex.com", "m.yandex.com":
		return u.Path == "/maps" || strings.HasPrefix(u.Path, "/maps/")
	case "maps.yandex.ru", "maps.yandex.com":
		return true
	default:
		return false
	}
}

func parseLonLat(text string) (*geoPoint, error) {
	// Explicit pins may append a style after lon,lat (e.g. pt=37,55,pm2rdm).
	parts := strings.Split(strings.TrimSpace(text), ",")
	if len(parts) < 2 {
		return nil, errMapPoint
	}
	lon, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	lat, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil || !validMapPoint(lat, lon) {
		return nil, errMapPoint
	}
	return &geoPoint{Latitude: lat, Longitude: lon}, nil
}

func coordsFromMapURL(u *url.URL) (*geoPoint, error) {
	if !allowedMapURL(u) {
		return nil, errMapPoint
	}
	params := u.Query()
	// A selected point is more precise than the map centre.
	for _, key := range []string{"whatshere[point]", "pt", "ll"} {
		if raw := params.Get(key); raw != "" {
			if point, err := parseLonLat(raw); err == nil {
				return point, nil
			}
		}
	}
	return nil, errMapPoint
}

// resolveSharedMapPoint accepts full links, short Yandex share URLs and
// coordinates. Redirects are restricted to an exact Yandex Maps allowlist
// to avoid following user-supplied URLs to arbitrary sites.
func resolveSharedMapPoint(ctx context.Context, baseClient *http.Client, input string) (*geoPoint, error) {
	if p, ok := parseCopiedCoordinates(input); ok {
		return p, nil
	}
	raw, found := sharedMapInput(input)
	if !found {
		return nil, errMapPoint
	}
	u, err := url.Parse(raw)
	if err != nil || !allowedMapURL(u) {
		return nil, errMapPoint
	}
	if p, err := coordsFromMapURL(u); err == nil {
		return p, nil
	}
	// Only short-share URLs need HTTP expansion; ordinary links without a
	// selected point should not cause extra network requests.
	if !strings.HasPrefix(u.Path, "/maps/-/") {
		return nil, errMapPoint
	}
	transport := http.DefaultTransport
	if baseClient != nil && baseClient.Transport != nil {
		transport = baseClient.Transport
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   6 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	for redirect := 0; redirect < 4; redirect++ {
		if !allowedMapURL(u) {
			return nil, errMapPoint
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, u.String(), nil)
		if err != nil {
			return nil, errMapPoint
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("yandex map link unavailable: %w", err)
		}
		location, status := resp.Header.Get("Location"), resp.StatusCode
		resp.Body.Close()
		if status < 300 || status >= 400 || location == "" {
			break
		}
		relative, err := url.Parse(location)
		if err != nil {
			break
		}
		next := u.ResolveReference(relative)
		if !allowedMapURL(next) {
			return nil, errMapPoint
		}
		u = next
		if p, err := coordsFromMapURL(u); err == nil {
			return p, nil
		}
	}
	return nil, errMapPoint
}
