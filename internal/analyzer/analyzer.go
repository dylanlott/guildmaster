package analyzer

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"

	elogo "github.com/kortemy/elo-go"
)

// default starting score used by the analyzer
const DefaultStartingScore = 1500

// FinalScore represents a player's final ranking and score.
type FinalScore struct {
	Player   string
	EloScore int
}

// InitializeElo returns a configured Elo engine.
func InitializeElo() *elogo.Elo {
	elo := elogo.NewElo()
	elo.D = 800
	elo.K = 40
	return elo
}

// ProcessScores reads the CSV at path and scores every game into the provided scores map.
// The map is mutated with absolute ratings.
func ProcessScores(path string, elo *elogo.Elo, scores map[string]int) error {
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
			if err := ScoreGame(elo, scores, game); err != nil {
				return fmt.Errorf("failed to score game: %w", err)
			}
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

// ScoreGame computes and applies Elo changes for a single game (winner first).
// scores map is mutated to hold absolute ratings (defaulting to DefaultStartingScore when absent).
func ScoreGame(elo *elogo.Elo, scores map[string]int, game []string) error {
	numPlayers := len(game)
	if numPlayers < 2 {
		return fmt.Errorf("invalid game: need at least 2 players, got %d", numPlayers)
	}

	// Ensure all players have a starting score and capture snapshot ratings.
	ratings := make([]float64, numPlayers)
	for i := 0; i < numPlayers; i++ {
		name := game[i]
		if _, exists := scores[name]; !exists {
			scores[name] = DefaultStartingScore
		}
		ratings[i] = float64(scores[name])
	}

	// Accumulate Elo deltas based on all pairwise outcomes from the snapshot.
	deltas := make([]float64, numPlayers)
	K := float64(elo.K)
	D := float64(elo.D)
	for i := 0; i < numPlayers; i++ {
		for j := i + 1; j < numPlayers; j++ {
			RA := ratings[i]
			RB := ratings[j]

			EA := 1.0 / (1.0 + math.Pow(10, (RB-RA)/D))

			deltaA := K * (1.0 - EA)
			deltaB := -deltaA

			deltas[i] += deltaA
			deltas[j] += deltaB
		}
	}

	// Apply accumulated deltas to absolute ratings.
	for i := 0; i < numPlayers; i++ {
		name := game[i]
		newRating := int(math.Round(ratings[i] + deltas[i]))
		scores[name] = newRating
	}
	return nil
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
