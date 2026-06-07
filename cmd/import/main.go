package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dylanlott/guildmaster/internal/analyzer"
	"github.com/dylanlott/guildmaster/internal/config"
	"github.com/dylanlott/guildmaster/internal/db"
	"github.com/dylanlott/guildmaster/internal/scoring"
	"github.com/dylanlott/guildmaster/internal/server"
)

func main() {
	source := flag.String("source", "", "game source label (defaults to csv or sheets based on import mode)")
	csvPath := flag.String("csv", "", "optional CSV file to import instead of Google Sheets")
	flag.Parse()

	cfg := config.Load()

	dbStore, err := db.NewStore(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("failed to open database %s: %v", cfg.DatabasePath, err)
	}
	defer func() {
		if err := dbStore.Close(); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	resolvedSource := strings.TrimSpace(*source)
	var games []*server.Game
	if strings.TrimSpace(*csvPath) != "" {
		if resolvedSource == "" {
			resolvedSource = "csv"
		}
		games, err = loadGamesFromCSV(*csvPath)
		if err != nil {
			log.Fatalf("failed to fetch games from csv: %v", err)
		}
	} else {
		if resolvedSource == "" {
			resolvedSource = "sheets"
		}
		games, err = server.FetchGameData(cfg.SpreadsheetID, cfg.ScoreboardAPIKey)
		if err != nil {
			if errors.Is(err, server.ErrSheetsNotConfigured) {
				log.Fatalf("failed to fetch games from sheets: %v (set SPREADSHEET_ID and SCOREBOARD_API_KEY, or pass -csv path/to/file.csv)", err)
			}
			log.Fatalf("failed to fetch games from sheets: %v", err)
		}
	}
	sort.Slice(games, func(i, j int) bool { return games[i].Timestamp.Before(games[j].Timestamp) })

	if err := dbStore.Reset(); err != nil {
		log.Fatalf("failed to reset database: %v", err)
	}

	scoreStore := scoring.NewStore()
	processed := 0
	for _, g := range games {
		if len(g.Rankings) < 2 {
			continue
		}

		snapshot := scoreStore.GetAll()
		deltas, err := scoring.ScoreGame(g.Rankings, scoring.DefaultK, scoring.DefaultD, snapshot)
		if err != nil {
			log.Fatalf("failed to score game %s: %v", g.ID, err)
		}
		scoring.ApplyDeltas(snapshot, deltas)
		scoreStore.ReplaceAll(snapshot)

		if _, err := dbStore.RecordGame(g.Timestamp, resolvedSource, g.Rankings, snapshot); err != nil {
			log.Fatalf("failed to persist game %s: %v", g.ID, err)
		}
		processed++
	}

	fmt.Printf("imported %d games, %d players into %s\n", processed, len(scoreStore.GetAll()), cfg.DatabasePath)
}

func loadGamesFromCSV(path string) ([]*server.Game, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	games := make([]*server.Game, 0)
	rowNum := 0
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read csv record: %w", err)
		}
		rowNum++
		if len(record) < 3 {
			continue
		}

		rankings := analyzer.ParseGame(record[2:])
		if len(rankings) < 2 {
			continue
		}

		date := strings.TrimSpace(record[1])
		timestamp, err := parseCSVDate(date)
		if err != nil {
			return nil, fmt.Errorf("parse csv date on row %d (%q): %w", rowNum, date, err)
		}

		games = append(games, &server.Game{
			ID:        fmt.Sprintf("csv-%d", rowNum),
			Date:      date,
			Timestamp: timestamp,
			Rankings:  rankings,
		})
	}

	return games, nil
}

func parseCSVDate(value string) (time.Time, error) {
	layouts := []string{
		"1/2/2006",
		"1/2/06",
		"2006-01-02",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, value); err == nil {
			return ts.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported date format")
}
