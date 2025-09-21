package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/dylanlott/guildmaster/internal/scoring"
	"github.com/dylanlott/guildmaster/internal/server"
)

func main() {
	addr := flag.String("addr", ":8080", "http listen address")
	staticDir := flag.String("static", "assets", "static assets directory")
	flag.Parse()

	store := scoring.NewStore()
	srv := server.New(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/scores", srv.HandleGetScores)
	mux.HandleFunc("/api/refresh", srv.HandleRefresh)
	mux.HandleFunc("/api/games", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			srv.HandleGetGames(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Serve the embedded landing page at / and static assets under /static/
	mux.HandleFunc("/", srv.HandleLanding)
	fs := http.FileServer(http.Dir(*staticDir))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// initial refresh to populate in-memory scores (ignore errors; landing page computes on the fly if needed)
	if err := srv.RefreshAndPersistScores(); err != nil {
		log.Printf("initial refresh failed: %v", err)
	}

	log.Printf("listening on %s, serving static from %s", *addr, *staticDir)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Printf("server failed: %v", err)
		os.Exit(1)
	}
}
