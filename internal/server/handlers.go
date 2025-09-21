package server

import (
	"embed"
	"encoding/json"
	"html/template"
	"net/http"
	"sort"

	"github.com/dylanlott/guildmaster/internal/analyzer"
	"github.com/dylanlott/guildmaster/internal/scoring"
)

type Server struct {
	store *scoring.Store
	K     int
	D     float64
}

func New(store *scoring.Store) *Server {
	return &Server{store: store, K: 40, D: 800}
}

// GET /api/scores - returns all current scores as JSON
func (s *Server) HandleGetScores(w http.ResponseWriter, r *http.Request) {
	scores := s.store.GetAll()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(scores)
}

//go:embed landing.tmpl
var tmplFS embed.FS

// computeScoresFromGames replays the games from Sheets (latest first) and returns a map of player->score
func (s *Server) computeScoresFromGames() (map[string]int, error) {
	games, err := fetchGameData()
	if err != nil {
		return nil, err
	}
	// replay games in chronological order (oldest first)
	sort.Slice(games, func(i, j int) bool { return games[i].Timestamp.Before(games[j].Timestamp) })
	// snapshot holds absolute ratings (1500 default)
	snapshot := make(map[string]int)
	elo := analyzer.InitializeElo()
	for _, g := range games {
		if len(g.Rankings) < 2 {
			continue
		}
		// reuse analyzer.ScoreGame which mutates the provided scores map
		if err := analyzer.ScoreGame(elo, snapshot, g.Rankings); err != nil {
			return nil, err
		}
	}
	return snapshot, nil
}

// HandleLanding renders the embedded landing template with computed scores and recent games
func (s *Server) HandleLanding(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFS(tmplFS, "landing.tmpl")
	if err != nil {
		http.Error(w, "template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	games, _ := fetchGameData() // ignore error here and show empty games if fails
	scores, _ := s.computeScoresFromGames()
	data := struct {
		Games  []*Game
		Scores map[string]int
	}{
		Games:  games,
		Scores: scores,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "template execute error: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
