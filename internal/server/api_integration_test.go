package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dylanlott/guildmaster/internal/config"
	"github.com/dylanlott/guildmaster/internal/db"
	"github.com/dylanlott/guildmaster/internal/scoring"
)

func TestAPIIntegrationGameLifecycle(t *testing.T) {
	dbStore := mustNewInMemoryDB(t)
	defer func() { _ = dbStore.Close() }()

	srv := New(scoring.NewStore(), dbStore, config.Config{GuildmasterAdminKey: "secret"})
	mux := newAPIMux(srv)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	playedAt := "2026-03-08T12:00:00Z"
	payload := map[string]interface{}{
		"rankings":  []string{"Alice", "Bob", "Cara"},
		"played_at": playedAt,
	}
	body, _ := json.Marshal(payload)

	postReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/games", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	postReq.Header.Set("Authorization", "Bearer secret")
	postReq.Header.Set("Content-Type", "application/json")

	postResp, err := http.DefaultClient.Do(postReq)
	if err != nil {
		t.Fatalf("post game: %v", err)
	}
	defer postResp.Body.Close()
	if postResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for post game, got %d", postResp.StatusCode)
	}

	listResp, err := http.Get(ts.URL + "/api/games")
	if err != nil {
		t.Fatalf("list games: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for list games, got %d", listResp.StatusCode)
	}
	var listed struct {
		Total int `json:"total"`
		Games []struct {
			ID       int64  `json:"id"`
			PlayedAt string `json:"played_at"`
		} `json:"games"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if listed.Total != 1 || len(listed.Games) != 1 {
		t.Fatalf("expected one game in list, got total=%d len=%d", listed.Total, len(listed.Games))
	}

	gotGameID := listed.Games[0].ID
	getResp, err := http.Get(fmt.Sprintf("%s/api/games/%d", ts.URL, gotGameID))
	if err != nil {
		t.Fatalf("get game by id: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for get by id, got %d", getResp.StatusCode)
	}

	playerResp, err := http.Get(ts.URL + "/api/players/Alice")
	if err != nil {
		t.Fatalf("get player: %v", err)
	}
	defer playerResp.Body.Close()
	if playerResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for player profile, got %d", playerResp.StatusCode)
	}
	var player struct {
		Name  string `json:"name"`
		Games []struct {
			PlayedAt string `json:"played_at"`
		} `json:"games"`
	}
	if err := json.NewDecoder(playerResp.Body).Decode(&player); err != nil {
		t.Fatalf("decode player profile: %v", err)
	}
	if player.Name != "Alice" || len(player.Games) != 1 {
		t.Fatalf("unexpected player profile: %+v", player)
	}
	if _, err := time.Parse(time.RFC3339, player.Games[0].PlayedAt); err != nil {
		t.Fatalf("expected RFC3339 played_at in profile game, got %q", player.Games[0].PlayedAt)
	}

	scoresResp, err := http.Get(ts.URL + "/api/scores")
	if err != nil {
		t.Fatalf("get scores: %v", err)
	}
	defer scoresResp.Body.Close()
	if scoresResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for scores, got %d", scoresResp.StatusCode)
	}
	var scores []struct {
		Name string `json:"name"`
		Elo  int    `json:"elo"`
	}
	if err := json.NewDecoder(scoresResp.Body).Decode(&scores); err != nil {
		t.Fatalf("decode scores: %v", err)
	}
	if len(scores) != 3 {
		t.Fatalf("expected 3 players in scores, got %d", len(scores))
	}
	if scores[0].Name != "Alice" {
		t.Fatalf("expected Alice to lead after win, got first=%s", scores[0].Name)
	}
}

func TestAPIRejectsUnauthorizedPostGame(t *testing.T) {
	dbStore := mustNewInMemoryDB(t)
	defer func() { _ = dbStore.Close() }()

	srv := New(scoring.NewStore(), dbStore, config.Config{GuildmasterAdminKey: "secret"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewBufferString(`{"rankings":["A","B"]}`))

	srv.HandleGames(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing bearer token, got %d", rec.Code)
	}
}

func TestAPIRejectsUnauthorizedRefresh(t *testing.T) {
	dbStore := mustNewInMemoryDB(t)
	defer func() { _ = dbStore.Close() }()

	srv := New(scoring.NewStore(), dbStore, config.Config{
		GuildmasterAdminKey: "secret",
	})
	mux := newAPIMux(srv)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/refresh", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer wrong")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("refresh request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid bearer token, got %d", resp.StatusCode)
	}
}

func TestAPIRefreshReturnsClearErrorWhenSheetsNotConfigured(t *testing.T) {
	dbStore := mustNewInMemoryDB(t)
	defer func() { _ = dbStore.Close() }()

	srv := New(scoring.NewStore(), dbStore, config.Config{
		GuildmasterAdminKey: "secret",
		SpreadsheetID:       "",
		ScoreboardAPIKey:    "",
	})
	mux := newAPIMux(srv)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/refresh", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("refresh request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 when sheets is not configured, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "Sheets not configured" {
		t.Fatalf("unexpected error message: %q", body["error"])
	}
}

func newAPIMux(srv *Server) *http.ServeMux {
	mux := http.NewServeMux()
	srv.RegisterAPIRoutes(mux)
	return mux
}

func mustNewInMemoryDB(t *testing.T) *db.Store {
	t.Helper()
	path := fmt.Sprintf("file:guildmaster_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	dbStore, err := db.NewStore(path)
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	return dbStore
}
