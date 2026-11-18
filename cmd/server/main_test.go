package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadOnlyAPI(t *testing.T) {
	tmp := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmp, "index.html"), []byte("<h1>demo</h1>"), 0600)
	h := routes(tmp)
	t.Run("locations", func(t *testing.T) {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/api/locations", nil))
		if r.Code != 200 {
			t.Fatal(r.Code)
		}
		var v struct {
			Locations []struct {
				ID string `json:"id"`
			} `json:"locations"`
			Demo bool `json:"demo"`
		}
		if e := json.Unmarshal(r.Body.Bytes(), &v); e != nil || !v.Demo || len(v.Locations) < 10 {
			t.Fatalf("invalid demo payload: %s; %v", r.Body.String(), e)
		}
	})
	t.Run("writes blocked", func(t *testing.T) {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/api/locations", strings.NewReader(`{}`)))
		if r.Code == 200 {
			t.Fatal("API should never accept writes")
		}
	})
	t.Run("spa", func(t *testing.T) {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/favorites", nil))
		if r.Code != 200 || !strings.Contains(r.Body.String(), "demo") {
			t.Fatalf("bad SPA fallback: %d", r.Code)
		}
	})
	t.Run("not found", func(t *testing.T) {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/api/unknown", nil))
		if r.Code != 404 {
			t.Fatal(r.Code)
		}
	})
}
