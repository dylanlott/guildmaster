package server

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/dylanlott/guildmaster/internal/scoring"
)

type gameHistoryRow struct {
	ID       int64
	PlayedAt string
	Source   string
	Players  string
	Count    int
}

type gameDetailPlayerRow struct {
	Placement int
	Player    string
	EloBefore int
	Delta     int
	EloAfter  int
}

func (s *Server) HandleGamesPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.dbStore == nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	if r.URL.Path != "/games" {
		http.NotFound(w, r)
		return
	}

	page := 1
	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			http.Error(w, "page must be a positive integer", http.StatusBadRequest)
			return
		}
		page = parsed
	}

	const limit = 25
	games, total, err := s.dbStore.ListGames(page, limit)
	if err != nil {
		http.Error(w, "failed to list games: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rows := make([]gameHistoryRow, 0, len(games))
	for _, game := range games {
		players := make([]string, 0, len(game.Placements))
		for _, placement := range game.Placements {
			players = append(players, placement.Player)
		}
		rows = append(rows, gameHistoryRow{
			ID:       game.ID,
			PlayedAt: humanDate(game.PlayedAt),
			Source:   game.Source,
			Players:  strings.Join(players, ", "),
			Count:    len(players),
		})
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	}
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		http.NotFound(w, r)
		return
	}

	t, err := template.New("games.tmpl").ParseFS(tmplFS, "games.tmpl")
	if err != nil {
		http.Error(w, "template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Games      []gameHistoryRow
		Page       int
		Total      int
		TotalPages int
		PrevPage   int
		NextPage   int
		HasPrev    bool
		HasNext    bool
		Nav        navData
	}{
		Games:      rows,
		Page:       page,
		Total:      total,
		TotalPages: totalPages,
		PrevPage:   page - 1,
		NextPage:   page + 1,
		HasPrev:    page > 1,
		HasNext:    page < totalPages,
		Nav:        navForRequest(r),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "template execute error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) HandleGameDetailPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.dbStore == nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}

	rawID := strings.TrimPrefix(r.URL.Path, "/games/")
	id, err := strconv.ParseInt(strings.TrimSpace(rawID), 10, 64)
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}

	game, err := s.dbStore.GetGameByID(id)
	if err != nil {
		http.Error(w, "failed to load game: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if game == nil {
		http.NotFound(w, r)
		return
	}

	replay, err := s.replayState()
	if err != nil {
		http.Error(w, "failed to replay games: "+err.Error(), http.StatusInternalServerError)
		return
	}

	snapshot := make(map[string]int)
	playerRows := make([]gameDetailPlayerRow, 0, len(game.Placements))
	for _, replayGame := range replay.Games {
		deltas := replay.DeltasByGame[replayGame.ID]
		if deltas == nil {
			continue
		}
		rankings := placementsToRankings(replayGame.Placements)
		for _, player := range rankings {
			if _, ok := snapshot[player]; !ok {
				snapshot[player] = scoring.DefaultStartingScore
			}
		}

		if replayGame.ID == id {
			for _, placement := range replayGame.Placements {
				before := snapshot[placement.Player]
				delta := deltas[placement.Player]
				playerRows = append(playerRows, gameDetailPlayerRow{
					Placement: placement.Placement,
					Player:    placement.Player,
					EloBefore: before,
					Delta:     delta,
					EloAfter:  before + delta,
				})
			}
			break
		}

		scoring.ApplyDeltas(snapshot, deltas)
	}

	t, err := template.New("game_detail.tmpl").Funcs(template.FuncMap{
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
	}).ParseFS(tmplFS, "game_detail.tmpl")
	if err != nil {
		http.Error(w, "template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		GameID   int64
		PlayedAt string
		Source   string
		Players  []gameDetailPlayerRow
		Nav      navData
	}{
		GameID:   game.ID,
		PlayedAt: humanDate(game.PlayedAt),
		Source:   game.Source,
		Players:  playerRows,
		Nav:      navForRequest(r),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "template execute error: "+err.Error(), http.StatusInternalServerError)
	}
}
