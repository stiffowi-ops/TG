package main

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"locationbot/internal/demo"
)

func routes(webDist string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		jsonReply(w, map[string]any{"ok": true, "mode": "demo"})
	})
	mux.HandleFunc("GET /api/locations", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=300")
		jsonReply(w, map[string]any{"locations": demo.Locations(), "demo": true})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		// filepath.Join with a cleaned relative name cannot escape webDist.
		clean := strings.TrimPrefix(filepath.Clean("/"+r.URL.Path), "/")
		if clean == "." || clean == "" {
			clean = "index.html"
		}
		path := filepath.Join(webDist, clean)
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			http.ServeFile(w, r, path)
			return
		}
		// Client-side routes use the SPA document, but missing assets stay 404.
		if filepath.Ext(clean) != "" {
			http.NotFound(w, r)
			return
		}
		index := filepath.Join(webDist, "index.html")
		if _, err := os.Stat(index); err != nil {
			http.Error(w, "web build not found: run npm run build", http.StatusServiceUnavailable)
			return
		}
		http.ServeFile(w, r, index)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		mux.ServeHTTP(w, r)
	})
}
func jsonReply(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(data)
}
func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	dist := os.Getenv("WEB_DIST")
	if dist == "" {
		dist = "web/dist"
	}
	if _, err := fs.Stat(os.DirFS("."), dist); err != nil {
		log.Printf("web build unavailable at %s (API is still accessible)", dist)
	}
	srv := &http.Server{Addr: addr, Handler: routes(dist), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("Mini App demo serving at %s", addr)
	log.Fatal(srv.ListenAndServe())
}
