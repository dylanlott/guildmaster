package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/dylanlott/guildmaster/internal/config"
	"github.com/dylanlott/guildmaster/internal/db"
	"github.com/dylanlott/guildmaster/internal/scoring"
	"github.com/dylanlott/guildmaster/internal/server"
)

func main() {
	staticDir := flag.String("static", "assets", "static assets directory")
	flag.Parse()

	cfg := config.Load()

	dbStore, err := db.NewStore(cfg.DatabasePath)
	if err != nil {
		log.Printf("failed to open database %s: %v", cfg.DatabasePath, err)
		os.Exit(1)
	}
	defer func() {
		if err := dbStore.Close(); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	store := scoring.NewStore()
	if existingScores, err := dbStore.LoadScores(); err != nil {
		log.Printf("failed to load scores from database: %v", err)
	} else {
		store.ReplaceAll(existingScores)
	}
	srv := server.New(store, dbStore, cfg)

	mux := http.NewServeMux()
	srv.RegisterAPIRoutes(mux)

	// Server-rendered pages.
	mux.HandleFunc("/register", srv.HandleRegister)
	mux.HandleFunc("/login", srv.HandleLogin)
	mux.HandleFunc("/logout", srv.HandleLogout)
	mux.HandleFunc("/pods/new", srv.RequireAuth(srv.HandlePodNew))
	mux.HandleFunc("/pods/", srv.HandlePods)
	mux.HandleFunc("/pods", srv.HandlePods)
	mux.HandleFunc("/players/", srv.HandlePlayerPage)
	mux.HandleFunc("/games/", srv.HandleGameDetailPage)
	mux.HandleFunc("/games", srv.HandleGamesPage)
	mux.HandleFunc("/submit/create", srv.HandleSubmitCreate)
	mux.HandleFunc("/submit", srv.HandleSubmitPage)
	mux.HandleFunc("/", srv.HandleLanding)

	// Static assets under /static/.
	fs := http.FileServer(http.Dir(*staticDir))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Initial refresh is optional and only runs when Sheets is configured.
	if cfg.SheetsConfigured() {
		if err := srv.RefreshAndPersistScores(); err != nil {
			log.Printf("initial refresh failed: %v", err)
		}
	}

	addr := cfg.ListenAddr()
	log.Printf("listening on %s, serving static from %s", addr, *staticDir)
	handler := srv.SessionMiddleware(mux)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Printf("server failed: %v", err)
		os.Exit(1)
	}
}
