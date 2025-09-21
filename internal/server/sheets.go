package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// Game mirrors the shape used in the previous app.go for Sheets-backed games.
type Game struct {
	ID        string    `json:"id"`
	Date      string    `json:"date"`
	Timestamp time.Time `json:"timestamp"`
	Rankings  []string  `json:"rankings"`
	TableZap  string    `json:"table_zap"`
	DrawGame  string    `json:"draw_game"`
}

const spreadsheetID = "1-qr-ejHx07Hrr35OymMcGRH00-Jzb-k8S8-xS9P5vqk"

// fetchGameData retrieves rows from Google Sheets and parses them into Game objects.
func fetchGameData() ([]*Game, error) {
	ctx := context.Background()
	key := os.Getenv("SCOREBOARD_API_KEY")
	srv, err := sheets.NewService(ctx, option.WithAPIKey(key))
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

//
// HTTP handlers
//

func (s *Server) HandleGetGames(w http.ResponseWriter, r *http.Request) {
	games, err := fetchGameData()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(games)
}
