package analyzer

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/dylanlott/guildmaster/internal/scoring"
)

const (
	// DefaultStartingScore is the fallback Elo for players with no prior games.
	DefaultStartingScore = scoring.DefaultStartingScore
)

// FinalScore represents a player's final ranking and score.
type FinalScore struct {
	Player   string
	EloScore int
}

// ProcessScores reads the CSV at path and scores every game into the provided scores map.
// The map is mutated with absolute ratings.
func ProcessScores(path string, scores map[string]int) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open scores file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading record: %w", err)
		}

		if len(record) < 3 {
			continue
		}

		game := ParseGame(record[2:])
		if len(game) >= 2 {
			deltas, err := scoring.ScoreGame(game, scoring.DefaultK, scoring.DefaultD, scores)
			if err != nil {
				return fmt.Errorf("failed to score game: %w", err)
			}
			scoring.ApplyDeltas(scores, deltas)
		}
	}
	return nil
}

// ParseGame turns CSV columns into an ordered slice of player names (winner first).
func ParseGame(players []string) []string {
	game := make([]string, 0, len(players))
	for _, player := range players {
		player = strings.TrimSpace(player)
		if player == "" {
			break
		}
		game = append(game, player)
	}
	return game
}

// CalculateFinalScores converts the scores map into a sorted slice of FinalScore
// ordered by Elo desc then player name asc.
func CalculateFinalScores(scores map[string]int) []FinalScore {
	finalScores := make([]FinalScore, 0, len(scores))
	for player, eloScore := range scores {
		finalScores = append(finalScores, FinalScore{Player: player, EloScore: eloScore})
	}

	// Deterministic ordering: stable sort by Elo desc, then player name asc as tiebreaker.
	sort.SliceStable(finalScores, func(i, j int) bool {
		if finalScores[i].EloScore == finalScores[j].EloScore {
			return finalScores[i].Player < finalScores[j].Player
		}
		return finalScores[i].EloScore > finalScores[j].EloScore
	})
	return finalScores
}
