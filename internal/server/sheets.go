package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

var ErrSheetsNotConfigured = errors.New("Sheets not configured")

// Game mirrors the shape used in the previous app.go for Sheets-backed games.
type Game struct {
	ID        string    `json:"id"`
	Date      string    `json:"date"`
	Timestamp time.Time `json:"timestamp"`
	Rankings  []string  `json:"rankings"`
	TableZap  string    `json:"table_zap"`
	DrawGame  string    `json:"draw_game"`
}

// FetchGameData retrieves rows from Google Sheets and parses them into Game objects.
func FetchGameData(spreadsheetID, apiKey string) ([]*Game, error) {
	if strings.TrimSpace(spreadsheetID) == "" {
		return nil, ErrSheetsNotConfigured
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, ErrSheetsNotConfigured
	}

	ctx := context.Background()
	srv, err := sheets.NewService(ctx, option.WithAPIKey(strings.TrimSpace(apiKey)))
	if err != nil {
		return nil, fmt.Errorf("failed to create sheets client: %w", err)
	}

	readRange := "Ranked game log!A:K"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve data from sheet: %w", err)
	}
	if len(resp.Values) == 0 {
		return nil, fmt.Errorf("no game data found")
	}
	return parseGameData(resp.Values)
}

func (s *Server) fetchGameData() ([]*Game, error) {
	return FetchGameData(s.cfg.SpreadsheetID, s.cfg.ScoreboardAPIKey)
}

// parseGameData converts the raw Sheets values into a slice of Game.
func parseGameData(values [][]interface{}) ([]*Game, error) {
	var games []*Game
	for idx, row := range values {
		if len(row) < 4 {
			continue
		}
		if idx == 0 {
			// header row
			continue
		}
		gameID := fmt.Sprintf("%v", row[0])
		date := fmt.Sprintf("%v", row[1])
		zap := fmt.Sprintf("%v", row[2])
		draw := fmt.Sprintf("%v", row[3])

		ts, _ := time.Parse(time.RFC1123, date)

		g := &Game{
			ID:        gameID,
			Date:      date,
			Timestamp: ts,
			Rankings:  []string{},
			TableZap:  zap,
			DrawGame:  draw,
		}

		// players start at column F (index 5)
		players := row[5:]
		for _, p := range players {
			name := strings.TrimSpace(fmt.Sprintf("%v", p))
			if name == "" {
				continue
			}
			// detect two-headed giant / team markers
			if strings.Contains(name, "/") {
				g.Rankings = nil
				break
			}
			g.Rankings = append(g.Rankings, name)
		}
		if g.Rankings == nil {
			// skip two-headed giant games for now
			continue
		}
		games = append(games, g)
	}
	return games, nil
}
