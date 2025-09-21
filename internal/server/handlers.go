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

// RefreshAndPersistScores recomputes scores from Sheets and persists them into the in-memory store.
func (s *Server) RefreshAndPersistScores() error {
	snapshot, err := s.computeScoresFromGames()
	if err != nil {
		return err
	}
	s.store.ReplaceAll(snapshot)
	return nil
}

// HandleRefresh recomputes and persists scores; returns the updated snapshot.
func (s *Server) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.RefreshAndPersistScores(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.store.GetAll())
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
	t, err := template.New("landing.tmpl").Funcs(template.FuncMap{
		"add1": func(i int) int { return i + 1 },
		"medal": func(i int) string {
			switch i {
			case 0:
				return "🥇"
			case 1:
				return "🥈"
			case 2:
				return "🥉"
			default:
				return ""
			}
		},
	}).ParseFS(tmplFS, "landing.tmpl")
	if err != nil {
		http.Error(w, "template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	games, _ := fetchGameData() // ignore error here and show empty games if fails
	// sort games by timestamp desc and keep only latest 10
	sort.Slice(games, func(i, j int) bool { return games[i].Timestamp.After(games[j].Timestamp) })
	if len(games) > 10 {
		games = games[:10]
	}

	scoresMap, _ := s.computeScoresFromGames()
	// build ranked slice sorted by score desc, then name asc for stability
	type scoreRow struct {
		Name  string
		Score int
	}
	ranked := make([]scoreRow, 0, len(scoresMap))
	for name, sc := range scoresMap {
		ranked = append(ranked, scoreRow{Name: name, Score: sc})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Score == ranked[j].Score {
			return ranked[i].Name < ranked[j].Name
		}
		return ranked[i].Score > ranked[j].Score
	})

	data := struct {
		Games       []*Game
		Ranked      []scoreRow
		PlayerCount int
	}{
		Games:       games,
		Ranked:      ranked,
		PlayerCount: len(ranked),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "template execute error: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
