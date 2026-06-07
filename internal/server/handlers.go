package server

import (
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"sort"
	"time"

	"github.com/dylanlott/guildmaster/internal/config"
	"github.com/dylanlott/guildmaster/internal/db"
	"github.com/dylanlott/guildmaster/internal/scoring"
)

type Server struct {
	store   scoring.SnapshotStore
	dbStore *db.Store
	cfg     config.Config
	K       int
	D       float64
}

func New(store scoring.SnapshotStore, dbStore *db.Store, cfg config.Config) *Server {
	return &Server{
		store:   store,
		dbStore: dbStore,
		cfg:     cfg,
		K:       scoring.DefaultK,
		D:       scoring.DefaultD,
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeAPIError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// GET /api/scores - returns all current scores as JSON (sorted desc).
func (s *Server) HandleGetScores(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	scores := s.sortedScores()
	writeJSON(w, http.StatusOK, scores)
}

// RefreshAndPersistScores recomputes scores from Sheets and persists them into the in-memory store.
func (s *Server) RefreshAndPersistScores() error {
	if !s.cfg.SheetsConfigured() {
		return ErrSheetsNotConfigured
	}
	games, err := s.fetchGameData()
	if err != nil {
		return err
	}
	snapshot, err := s.replayGames(games, true, "sheets")
	if err != nil {
		return err
	}
	s.store.ReplaceAll(snapshot)
	return nil
}

// HandleRefresh recomputes and persists scores; returns the updated snapshot.
func (s *Server) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorizeAdminRequest(w, r) {
		return
	}
	if !s.cfg.SheetsConfigured() {
		writeAPIError(w, http.StatusBadRequest, ErrSheetsNotConfigured.Error())
		return
	}
	if err := s.RefreshAndPersistScores(); err != nil {
		if errors.Is(err, ErrSheetsNotConfigured) {
			writeAPIError(w, http.StatusBadRequest, ErrSheetsNotConfigured.Error())
			return
		}
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.sortedScores())
}

func (s *Server) sortedScores() interface{} {
	if s.dbStore != nil {
		scores, err := s.dbStore.GetSortedScores()
		if err == nil {
			return scores
		}
	}

	raw := s.store.GetAll()
	type scoreRow struct {
		Name string `json:"name"`
		Elo  int    `json:"elo"`
	}
	scores := make([]scoreRow, 0, len(raw))
	for name, elo := range raw {
		scores = append(scores, scoreRow{Name: name, Elo: elo})
	}
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].Elo == scores[j].Elo {
			return scores[i].Name < scores[j].Name
		}
		return scores[i].Elo > scores[j].Elo
	})
	return scores
}

//go:embed *.tmpl
var tmplFS embed.FS

// computeScoresFromGames replays the games from Sheets (latest first) and returns a map of player->score
func (s *Server) computeScoresFromGames() (map[string]int, error) {
	if !s.cfg.SheetsConfigured() {
		return s.store.GetAll(), nil
	}
	games, err := s.fetchGameData()
	if err != nil {
		return nil, err
	}
	return s.replayGames(games, false, "")
}

func (s *Server) replayGames(games []*Game, persist bool, source string) (map[string]int, error) {
	sort.Slice(games, func(i, j int) bool { return games[i].Timestamp.Before(games[j].Timestamp) })

	if persist && s.dbStore != nil {
		if err := s.dbStore.Reset(); err != nil {
			return nil, err
		}
	}

	snapshot := make(map[string]int)
	for _, g := range games {
		if len(g.Rankings) < 2 {
			continue
		}
		deltas, err := scoring.ScoreGame(g.Rankings, s.K, s.D, snapshot)
		if err != nil {
			return nil, err
		}
		scoring.ApplyDeltas(snapshot, deltas)

		if persist && s.dbStore != nil {
			if _, err := s.dbStore.RecordGame(g.Timestamp, source, g.Rankings, snapshot); err != nil {
				return nil, err
			}
		}
	}
	return snapshot, nil
}

type leaderboardRow struct {
	Rank        int
	Name        string
	Elo         int
	Delta       int
	GamesPlayed int
	Wins        int
	WinRate     float64
}

func (s *Server) buildLeaderboardRows() ([]leaderboardRow, error) {
	if s.dbStore == nil {
		return nil, nil
	}

	scores, err := s.dbStore.GetSortedScores()
	if err != nil {
		return nil, err
	}

	playerStats, err := s.dbStore.GetPlayerStats()
	if err != nil {
		return nil, err
	}

	allGames, err := s.dbStore.ListAllGamesChronological()
	if err != nil {
		return nil, err
	}

	snapshot := make(map[string]int)
	lastDelta := make(map[string]int)

	for _, game := range allGames {
		rankings := make([]string, 0, len(game.Placements))
		for _, placement := range game.Placements {
			rankings = append(rankings, placement.Player)
		}
		if len(rankings) < 2 {
			continue
		}
		deltas, err := scoring.ScoreGame(rankings, s.K, s.D, snapshot)
		if err != nil {
			return nil, err
		}
		scoring.ApplyDeltas(snapshot, deltas)
		for name, delta := range deltas {
			lastDelta[name] = delta
		}
	}

	rows := make([]leaderboardRow, 0, len(scores))
	for _, score := range scores {
		row := leaderboardRow{
			Name:  score.Name,
			Elo:   score.Elo,
			Delta: lastDelta[score.Name],
		}
		if stats, ok := playerStats[score.Name]; ok {
			row.GamesPlayed = stats.TotalGames
			row.Wins = stats.Wins
			row.WinRate = stats.WinRate
		}
		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Elo == rows[j].Elo {
			return rows[i].Name < rows[j].Name
		}
		return rows[i].Elo > rows[j].Elo
	})
	for i := range rows {
		rows[i].Rank = i + 1
	}

	return rows, nil
}

func humanDate(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04 UTC")
}

// HandleLanding renders the leaderboard page.
func (s *Server) HandleLanding(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	t, err := template.New("landing.tmpl").Funcs(template.FuncMap{
		"deltaClass": func(v int) string {
			switch {
			case v > 0:
				return "pos"
			case v < 0:
				return "neg"
			default:
				return "zero"
			}
		},
		"humanDate": humanDate,
	}).ParseFS(tmplFS, "landing.tmpl")
	if err != nil {
		http.Error(w, "template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rows, err := s.buildLeaderboardRows()
	if err != nil {
		http.Error(w, "failed to build leaderboard: "+err.Error(), http.StatusInternalServerError)
		return
	}

	games := make([]db.GameRecord, 0, 10)
	if s.dbStore != nil {
		recentGames, _, err := s.dbStore.ListGames(1, 10)
		if err == nil {
			games = recentGames
		}
	}

	data := struct {
		Games       []db.GameRecord
		Ranked      []leaderboardRow
		PlayerCount int
		Nav         navData
	}{
		Games:       games,
		Ranked:      rows,
		PlayerCount: len(rows),
		Nav:         navForRequest(r),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "template execute error: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
