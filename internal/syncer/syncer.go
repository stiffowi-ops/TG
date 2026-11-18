package syncer

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"locationbot/internal/catalog"
)

const maxImageBytes = 18 << 20
const apiBase = "https://cloud-api.yandex.net/v1/disk/public"

var numPrefix = regexp.MustCompile(`^(\d{3})(?:\s|_|-)`)

type diskItem struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Path     string `json:"path"`
	Modified string `json:"modified"`
	Size     int64  `json:"size"`
}
type listResponse struct {
	Embedded struct {
		Items []diskItem `json:"items"`
	} `json:"_embedded"`
}
type downloadResponse struct {
	Href string `json:"href"`
}
type Result struct {
	Total, Downloaded, Skipped int
	Failures                   []string
}

type Syncer struct {
	client    *http.Client
	publicKey string
	store     *catalog.Store
	mu        sync.Mutex
}

func New(store *catalog.Store, publicKey string) *Syncer {
	return &Syncer{store: store, publicKey: publicKey, client: &http.Client{Timeout: 65 * time.Second}}
}
func imageExt(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if catalog.ValidExt(ext) {
		return ext
	}
	return ""
}
func locationID(name string) string {
	if m := numPrefix.FindStringSubmatch(name); len(m) > 1 {
		return m[1]
	}
	h := sha256.Sum256([]byte(strings.ToLower(name)))
	return fmt.Sprintf("file-%x", h[:5])
}
func Category(name string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(name)), "метро ") {
		return "Метро"
	}
	return "Другие локации"
}
func Title(name string) string {
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = numPrefix.ReplaceAllString(name, "")
	return strings.TrimSpace(name)
}
func (s *Syncer) getJSON(ctx context.Context, endpoint string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	res, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("Yandex HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(b)))
	}
	return json.NewDecoder(io.LimitReader(res.Body, 5<<20)).Decode(into)
}
func (s *Syncer) list(ctx context.Context) ([]diskItem, error) {
	var out []diskItem
	const pageSize = 100
	for offset := 0; offset < 10000; offset += pageSize {
		u, _ := url.Parse(apiBase + "/resources")
		q := u.Query()
		q.Set("public_key", s.publicKey)
		q.Set("limit", fmt.Sprint(pageSize))
		q.Set("offset", fmt.Sprint(offset))
		u.RawQuery = q.Encode()
		var result listResponse
		if err := s.getJSON(ctx, u.String(), &result); err != nil {
			return nil, err
		}
		out = append(out, result.Embedded.Items...)
		if len(result.Embedded.Items) < pageSize {
			return out, nil
		}
	}
	return nil, errors.New("too many Yandex Disk files; refusing unbounded sync")
}
func (s *Syncer) download(ctx context.Context, item diskItem, id, ext string) error {
	u, _ := url.Parse(apiBase + "/resources/download")
	q := u.Query()
	q.Set("public_key", s.publicKey)
	// The configured public folder contains files at its root. Public-resource
	// paths are relative to that public share, not disk:/ paths of the owner.
	q.Set("path", "/"+item.Name)
	u.RawQuery = q.Encode()
	var r downloadResponse
	if err := s.getJSON(ctx, u.String(), &r); err != nil {
		return err
	}
	link, err := url.Parse(r.Href)
	if err != nil || link.Scheme != "https" || !(link.Hostname() == "downloader.disk.yandex.ru" || strings.HasSuffix(link.Hostname(), ".disk.yandex.ru")) {
		return errors.New("Yandex returned unexpected download host")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link.String(), nil)
	if err != nil {
		return err
	}
	res, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("download HTTP %d", res.StatusCode)
	}
	if res.ContentLength > maxImageBytes {
		return errors.New("image too large")
	}
	imgdir := filepath.Join(s.store.DataDir(), "images")
	f, err := os.CreateTemp(imgdir, ".image-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	n, err := io.Copy(f, io.LimitReader(res.Body, maxImageBytes+1))
	if err != nil {
		f.Close()
		return err
	}
	if n > maxImageBytes {
		f.Close()
		return errors.New("image too large")
	}
	if n < 8 {
		f.Close()
		return errors.New("empty download")
	}
	if _, err = f.Seek(0, 0); err != nil {
		f.Close()
		return err
	}
	sniff := make([]byte, 512)
	count, _ := f.Read(sniff)
	contentType := http.DetectContentType(sniff[:count])
	if !strings.HasPrefix(contentType, "image/") {
		f.Close()
		return fmt.Errorf("expected image, got %s", contentType)
	}
	if err = f.Chmod(0644); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(imgdir, id+ext))
}

var ErrBusy = errors.New("catalog sync is already running")

func (s *Syncer) TryRun(ctx context.Context) (Result, error) {
	if !s.mu.TryLock() {
		return Result{}, ErrBusy
	}
	defer s.mu.Unlock()
	return s.run(ctx)
}

func (s *Syncer) Run(ctx context.Context) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.run(ctx)
}

func (s *Syncer) run(ctx context.Context) (Result, error) {
	result := Result{}
	items, err := s.list(ctx)
	if err != nil {
		return result, err
	}
	old, err := s.store.Base()
	if err != nil {
		return result, err
	}
	previous := make(map[string]catalog.Location)
	for _, p := range old {
		previous[p.ID] = p
	}
	updated := make([]catalog.Location, 0, len(items))
	for _, it := range items {
		if it.Type != "file" || imageExt(it.Name) == "" {
			continue
		}
		result.Total++
		id, ext := locationID(it.Name), imageExt(it.Name)
		loc := catalog.Location{ID: id, Title: Title(it.Name), Filename: it.Name, ImageExt: ext, Category: Category(Title(it.Name)), SourceModified: it.Modified}
		prev, exists := previous[id]
		if exists {
			loc.Description = prev.Description
			loc.Rating = prev.Rating
			loc.Landmarks = prev.Landmarks
			loc.LineStatus = prev.LineStatus
			loc.Traffic = prev.Traffic
			loc.Payers = prev.Payers
			loc.Opportunity = prev.Opportunity
			loc.Conclusion = prev.Conclusion
			loc.MapURL = prev.MapURL
			loc.Latitude = prev.Latitude
			loc.Longitude = prev.Longitude
		}
		path := filepath.Join(s.store.DataDir(), "images", id+ext)
		_, staterr := os.Stat(path)
		if exists && prev.SourceModified == it.Modified && staterr == nil {
			result.Skipped++
		} else if it.Size > maxImageBytes {
			result.Failures = append(result.Failures, fmt.Sprintf("%s: too large", it.Name))
			if exists {
				loc.SourceModified = prev.SourceModified
			}
		} else if err := s.download(ctx, it, id, ext); err != nil {
			result.Failures = append(result.Failures, fmt.Sprintf("%s: %s", it.Name, err))
			// Preserve the previous version so the next sync retries an update.
			if exists {
				loc.SourceModified = prev.SourceModified
			}
		} else {
			result.Downloaded++
		}
		updated = append(updated, loc)
	}
	if result.Total == 0 {
		return result, errors.New("Yandex Disk folder has no supported images; keeping existing catalog")
	}
	if err = s.store.SaveBase(updated); err != nil {
		return result, err
	}
	if err = s.store.Reload(); err != nil {
		return result, err
	}
	return result, nil
}
