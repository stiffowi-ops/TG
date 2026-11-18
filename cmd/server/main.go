package main

import (
	"context"
	"os/signal"
	"syscall"
	"locationbot/internal/catalog"
	"locationbot/internal/photosync"
	"locationbot/internal/syncer"
	"locationbot/internal/telegram"
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
	if addr == "" { addr = ":8080" }
	dist := os.Getenv("WEB_DIST")
	if dist == "" { dist = "web/dist" }
	if _, err := fs.Stat(os.DirFS("."), dist); err != nil {
		log.Printf("web build unavailable at %s", dist)
	}
	token := strings.TrimSpace(os.Getenv("BOT_TOKEN"))
	if token == "" {token = strings.TrimSpace(os.Getenv("TEST_BOT_TOKEN"))}
	dataDir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if dataDir == "" {
		if token=="" {dataDir = filepath.Join(os.TempDir(),"miniapp-demo")}
		if token!="" {dataDir = "/app/runtime"}
	}
	if err := os.MkdirAll(dataDir,0700);err!=nil {log.Fatalf("runtime directory: %v",err)}
	seedPath := filepath.Join(dataDir,"seed.json")
	if _,err:=os.Stat(seedPath);os.IsNotExist(err){
		src,readErr:=os.ReadFile("config/seed.json")
		if readErr!=nil {log.Fatalf("seed read: %v",readErr)}
		if err:=os.WriteFile(seedPath,src,0600);err!=nil {log.Fatalf("seed write: %v",err)}
	}
	overridePath:=os.Getenv("OVERRIDES_PATH")
	if overridePath=="" { overridePath=filepath.Join(dataDir,"overrides.json") }
	store,err:=catalog.NewWithBundled(dataDir,seedPath,overridePath)
	if err!=nil {log.Fatalf("catalog: %v",err)}
	photoService,err:=photosync.New(store,os.Getenv("YANDEX_PHOTOS_URL"))
	if err!=nil {log.Printf("photo sync unavailable: %v",err)}
	var bot *telegram.Bot
	ctx,stop:=signal.NotifyContext(context.Background(),syscall.SIGTERM,syscall.SIGINT)
	defer stop()
	if token!="" {
		syncerService:=syncer.New(store,os.Getenv("YANDEX_DISK_PUBLIC_URL"))
		bot,err=telegram.New(token,os.Getenv("MAP_CONSTRUCTOR_ID"),dataDir,store,syncerService,os.Getenv("ADMIN_IDS"))
		if err!=nil {log.Fatalf("telegram: %v",err)}
		go func(){
			if err:=bot.Run(ctx);err!=nil&&ctx.Err()==nil {log.Fatalf("telegram bot stopped: %v",err)}
		}()
		log.Printf("Telegram long-polling and Mini App are sharing one data store")
	}else{
		log.Printf("BOT_TOKEN is missing; full catalog works, private actions require a test bot")
	}
	if photoService!=nil {
		photoService.Start(ctx)
		log.Printf("Yandex gallery synchronization enabled")
	}
	srv:=&http.Server{Addr:addr,Handler:(&appService{store:store,bot:bot,token:token,photos:photoService}).handler(dist),ReadHeaderTimeout:5*time.Second}
	go func(){<-ctx.Done();ctx2,cancel:=context.WithTimeout(context.Background(),10*time.Second);defer cancel();_ = srv.Shutdown(ctx2)}()
	log.Printf("Mini App serving at %s; locations=%d",addr,len(store.List()))
	if err:=srv.ListenAndServe();err!=nil&&err!=http.ErrServerClosed{log.Fatal(err)}
}
