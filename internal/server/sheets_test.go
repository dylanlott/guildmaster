package server

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseGameDataFromCSVFixture(t *testing.T) {
	rows := loadCSVFixture(t, "fixtures/sheets_rows.csv")
	games, err := parseGameData(rows)
	if err != nil {
		t.Fatalf("parseGameData failed: %v", err)
	}
	assertParsedGames(t, games)
}

func TestParseGameDataFromJSONFixture(t *testing.T) {
	rows := loadJSONFixture(t, "fixtures/sheets_rows.json")
	games, err := parseGameData(rows)
	if err != nil {
		t.Fatalf("parseGameData failed: %v", err)
	}
	assertParsedGames(t, games)
}

func loadCSVFixture(t *testing.T, relPath string) [][]interface{} {
	t.Helper()
	f, err := os.Open(filepath.Clean(relPath))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("read fixture csv: %v", err)
	}

	out := make([][]interface{}, 0, len(records))
	for _, rec := range records {
		row := make([]interface{}, 0, len(rec))
		for _, col := range rec {
			row = append(row, col)
		}
		out = append(out, row)
	}
	return out
}

func loadJSONFixture(t *testing.T, relPath string) [][]interface{} {
	t.Helper()
	b, err := os.ReadFile(filepath.Clean(relPath))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var raw [][]string
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal fixture json: %v", err)
	}

	out := make([][]interface{}, 0, len(raw))
	for _, rec := range raw {
		row := make([]interface{}, 0, len(rec))
		for _, col := range rec {
			row = append(row, col)
		}
		out = append(out, row)
	}
	return out
}

func assertParsedGames(t *testing.T, games []*Game) {
	t.Helper()
	if len(games) != 2 {
		t.Fatalf("expected 2 parsed games, got %d", len(games))
	}

	if games[0].ID != "g1" {
		t.Fatalf("expected first game id g1, got %s", games[0].ID)
	}
	if len(games[0].Rankings) != 3 || games[0].Rankings[0] != "Alice" || games[0].Rankings[2] != "Carol" {
		t.Fatalf("unexpected rankings for g1: %#v", games[0].Rankings)
	}
	if games[0].Timestamp.IsZero() {
		t.Fatal("expected g1 timestamp to parse")
	}

	if games[1].ID != "g3" {
		t.Fatalf("expected second game id g3, got %s", games[1].ID)
	}
	if len(games[1].Rankings) != 2 || games[1].Rankings[1] != "Erin" {
		t.Fatalf("unexpected rankings for g3: %#v", games[1].Rankings)
	}
	if !games[1].Timestamp.Equal(time.Time{}) {
		t.Fatalf("expected zero timestamp for invalid date, got %v", games[1].Timestamp)
	}
}
