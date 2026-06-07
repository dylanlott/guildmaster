package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dylanlott/guildmaster/internal/scoring"
)

type postGameRequest struct {
	Rankings []string `json:"rankings"`
	Players  []string `json:"players"`
	PlayedAt string   `json:"played_at"`
}

func normalizeRankings(rankings []string) []string {
	out := make([]string, 0, len(rankings))
	for _, name := range rankings {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func (s *Server) HandleGetPlayer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.dbStore == nil {
		writeAPIError(w, http.StatusInternalServerError, "database unavailable")
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/api/players/")
	if name == "" {
		writeAPIError(w, http.StatusBadRequest, "player name is required")
		return
	}
	decodedName, err := url.PathUnescape(name)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid player name")
		return
	}

	profile, err := s.dbStore.GetPlayerProfile(decodedName)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if profile == nil {
		writeAPIError(w, http.StatusNotFound, "player not found")
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) HandleGames(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListGames(w, r)
	case http.MethodPost:
		s.handlePostGame(w, r)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleListGames(w http.ResponseWriter, r *http.Request) {
	if s.dbStore == nil {
		writeAPIError(w, http.StatusInternalServerError, "database unavailable")
		return
	}

	page := 1
	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeAPIError(w, http.StatusBadRequest, "page must be a positive integer")
			return
		}
		page = parsed
	}

	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeAPIError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		if parsed > 100 {
			parsed = 100
		}
		limit = parsed
	}

	games, total, err := s.dbStore.ListGames(page, limit)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"page":  page,
		"limit": limit,
		"total": total,
		"games": games,
	})
}

func (s *Server) HandleGetGameByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.dbStore == nil {
		writeAPIError(w, http.StatusInternalServerError, "database unavailable")
		return
	}

	rawID := strings.TrimPrefix(r.URL.Path, "/api/games/")
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id < 1 {
		writeAPIError(w, http.StatusBadRequest, "invalid game id")
		return
	}

	game, err := s.dbStore.GetGameByID(id)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if game == nil {
		writeAPIError(w, http.StatusNotFound, "game not found")
		return
	}

	writeJSON(w, http.StatusOK, game)
}

func (s *Server) handlePostGame(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeAdminRequest(w, r) {
		return
	}
	if s.dbStore == nil {
		writeAPIError(w, http.StatusInternalServerError, "database unavailable")
		return
	}

	var req postGameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	rankings := req.Rankings
	if len(rankings) == 0 {
		rankings = req.Players
	}
	rankings = normalizeRankings(rankings)
	if len(rankings) < 2 {
		writeAPIError(w, http.StatusBadRequest, "at least two ranked players are required")
		return
	}

	playedAt := time.Now().UTC()
	if strings.TrimSpace(req.PlayedAt) != "" {
		parsed, err := time.Parse(time.RFC3339, req.PlayedAt)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "played_at must be RFC3339")
			return
		}
		playedAt = parsed.UTC()
	}

	gameID, deltas, snapshot, err := s.recordGame(rankings, playedAt, "api")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}

	game, err := s.dbStore.GetGameByID(gameID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"game":   game,
		"deltas": deltas,
		"scores": snapshot,
	})
}

func (s *Server) recordGame(rankings []string, playedAt time.Time, source string) (int64, map[string]int, map[string]int, error) {
	if s.dbStore == nil {
		return 0, nil, nil, errors.New("database unavailable")
	}

	snapshot := s.store.GetAll()
	deltas, err := scoring.ScoreGame(rankings, s.K, s.D, snapshot)
	if err != nil {
		return 0, nil, nil, err
	}
	scoring.ApplyDeltas(snapshot, deltas)
	gameID, err := s.dbStore.RecordGame(playedAt, source, rankings, snapshot)
	if err != nil {
		return 0, nil, nil, err
	}
	s.store.ReplaceAll(snapshot)
	return gameID, deltas, snapshot, nil
}

func (s *Server) HandleAPINotFound(w http.ResponseWriter, r *http.Request) {
	writeAPIError(w, http.StatusNotFound, "not found")
}
