package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"sort"
	"strings"

	elogo "github.com/kortemy/elo-go"
)

// The default starting score of players in our analyzer.
const defaultStartingScore = 1500

type finalScore struct {
	player   string
	eloScore int
}

func main() {
	path := flag.String("path", "./mtgscores.csv", "path to analyze with tracker")
	useTUI := flag.Bool("tui", false, "use terminal UI for displaying rankings")
	flag.Parse()

	// Suppress logs during TUI mode to prevent interference with display
	if *useTUI {
		log.SetOutput(io.Discard)
	}

	log.Printf("Analyzing scores for %s", *path)

	elo := initializeElo()
	scores := make(map[string]int)

	if err := processScores(*path, elo, scores); err != nil {
		if *useTUI {
			log.SetOutput(os.Stderr) // Restore for error display
		}
		log.Fatalf("Error processing scores: %v", err)
	}

	finalScores := calculateFinalScores(scores)

	if *useTUI {
		// Use the TUI to display rankings
		if err := DisplayRankingsTUI(finalScores); err != nil {
			log.SetOutput(os.Stderr) // Restore log output to show errors
			log.Fatalf("Error in TUI: %v", err)
		}
	} else {
		// Use the original console output
		for i, v := range finalScores {
			fmt.Printf("%d --- %s --- %d\n", i+1, v.player, v.eloScore)
		}
	}
}

func initializeElo() *elogo.Elo {
	elo := elogo.NewElo()
	elo.D = 800
	elo.K = 40
	return elo
}

func processScores(path string, elo *elogo.Elo, scores map[string]int) error {
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

		game := parseGame(record[2:])
		if len(game) >= 2 {
			if err := scoreGame(elo, scores, game); err != nil {
				return fmt.Errorf("failed to score game: %w", err)
			}
		}
	}
	return nil
}

func parseGame(players []string) []string {
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

func scoreGame(elo *elogo.Elo, scores map[string]int, game []string) error {
	numPlayers := len(game)
	if numPlayers < 2 {
		return fmt.Errorf("invalid game: need at least 2 players, got %d", numPlayers)
	}

	// Ensure all players have a starting score and capture snapshot ratings.
	ratings := make([]float64, numPlayers)
	for i := 0; i < numPlayers; i++ {
		name := game[i]
		if _, exists := scores[name]; !exists {
			scores[name] = defaultStartingScore
		}
		ratings[i] = float64(scores[name])
	}

	// Accumulate Elo deltas based on all pairwise outcomes from the snapshot.
	deltas := make([]float64, numPlayers)
	K := float64(elo.K)
	D := float64(elo.D)
	for i := range numPlayers {
		for j := i + 1; j < numPlayers; j++ {
			// Player at index i placed higher than player at index j, so i wins vs j.
			RA := ratings[i]
			RB := ratings[j]

			// Expected score of A vs B
			EA := 1.0 / (1.0 + math.Pow(10, (RB-RA)/D))

			// Single-game deltas
			deltaA := K * (1.0 - EA) // A wins
			deltaB := -deltaA        // B loses

			// Apply deltas
			deltas[i] += deltaA
			deltas[j] += deltaB
		}
	}

	// Apply accumulated deltas.
	for i := range numPlayers {
		name := game[i]
		newRating := int(math.Round(ratings[i] + deltas[i]))
		scores[name] = newRating
	}
	return nil
}

func calculateFinalScores(scores map[string]int) []finalScore {
	finalScores := make([]finalScore, 0, len(scores))
	for player, eloScore := range scores {
		finalScores = append(finalScores, finalScore{player: player, eloScore: eloScore})
	}

	// Deterministic ordering: stable sort by Elo desc, then player name asc as tiebreaker.
	sort.SliceStable(finalScores, func(i, j int) bool {
		if finalScores[i].eloScore == finalScores[j].eloScore {
			return finalScores[i].player < finalScores[j].player
		}
		return finalScores[i].eloScore > finalScores[j].eloScore
	})

	return finalScores
}

// displayScores was previously used for console output; removed to avoid unused warnings.
