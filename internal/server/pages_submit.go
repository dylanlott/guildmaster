package server

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Server) HandleSubmitPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := UserFromContext(r.Context())
	canSubmit := s.userCanAdmin(user)

	players := make([]string, 0)
	if canSubmit && s.dbStore != nil {
		scores, err := s.dbStore.GetSortedScores()
		if err == nil {
			players = make([]string, 0, len(scores))
			for _, score := range scores {
				players = append(players, score.Name)
			}
		}
	}

	t, err := template.New("submit.tmpl").ParseFS(tmplFS, "submit.tmpl")
	if err != nil {
		http.Error(w, "template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		CanSubmit bool
		Players   []string
		NowLocal  string
		Nav       navData
	}{
		CanSubmit: canSubmit,
		Players:   players,
		NowLocal:  time.Now().Format("2006-01-02T15:04"),
		Nav:       navForRequest(r),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "template execute error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) HandleSubmitCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := UserFromContext(r.Context())
	if !s.userCanAdmin(user) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !s.validateCSRF(r) {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	rankings := normalizeRankings(r.Form["placement"])
	if len(rankings) < 2 {
		http.Error(w, "at least two ranked players are required", http.StatusBadRequest)
		return
	}

	seen := make(map[string]struct{}, len(rankings))
	for _, player := range rankings {
		if _, ok := seen[player]; ok {
			http.Error(w, "duplicate players are not allowed", http.StatusBadRequest)
			return
		}
		seen[player] = struct{}{}
	}

	playedAt := time.Now().UTC()
	if raw := strings.TrimSpace(r.FormValue("played_at")); raw != "" {
		parsed, err := time.Parse("2006-01-02T15:04", raw)
		if err != nil {
			http.Error(w, "played_at must be a valid datetime", http.StatusBadRequest)
			return
		}
		playedAt = parsed.UTC()
	}

	gameID, _, _, err := s.recordGame(rankings, playedAt, "submit")
	if err != nil {
		http.Error(w, "failed to submit game: "+err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "/games/"+strconv.FormatInt(gameID, 10), http.StatusSeeOther)
}
