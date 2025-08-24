package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	elogo "github.com/kortemy/elo-go"
)

// The default starting score of players in our analyzer.
var defaultStartingScore = 1500

type finalScore struct {
	player   string
	eloScore int
}

func main() {
	filepath := flag.String("path", "./mtgscores.csv", "path to analyze with tracker")
	useTUI := flag.Bool("tui", false, "use terminal UI for displaying rankings")
	window := flag.Int("window", 0, "only consider the last N games (0 = all games)")
	flag.Parse()

	log.Printf("analyzing scores for %s", *filepath)

	finalScores, err := processGamesFromFile(*filepath, *window)
	if err != nil {
		log.Fatalf("failed to process games: %v", err)
	}

	// print out final scores
	if *useTUI {
		// Use the TUI to display rankings
		for {
			err := DisplayRankingsTUI(finalScores)
			if err != nil {
				log.Fatalf("error in TUI: %v", err)
			}

			// Check if we need to reload rankings (this happens when a new game is added)
			finalScores, err = processGamesFromFile(*filepath, *window)
			if err != nil {
				log.Fatalf("failed to reload games: %v", err)
			}

			log.Printf("Rankings reloaded with %d players", len(finalScores))
		}
	} else {
		// Use the original console output
		for i, v := range finalScores {
			fmt.Printf("%d --- %s --- %d\n", i, v.player, v.eloScore)
		}
	}

	// log.Printf("player scores %v", scores)
}

// processGamesFromFile reads a CSV file and calculates Elo ratings for all games
// If window > 0, only considers the last 'window' number of games
func processGamesFromFile(filepath string, window int) ([]finalScore, error) {
	elo := elogo.NewElo()
	elo.D = 800
	elo.K = 40

	scores := map[string]int{}

	f, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open scores: %v", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)

	// Skip the header row
	_, err = reader.Read()
	if err != nil {
		return nil, fmt.Errorf("error reading header: %v", err)
	}

	// Read all games first to implement windowing
	var allGames [][]string
	for {
		rec, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("error processing record: %v", err)
		}

		players := rec[5:] // Player rankings start at column 5 (1st Place, 2nd Place, etc.)
		
		// Process the entire row as one complete game
		currentGame := []string{}
		seen := make(map[string]bool)
		for _, player := range players {
			player = strings.TrimSpace(player)
			if player == "" {
				break // Stop when we hit empty cells
			}
			// Skip duplicate players
			if seen[player] {
				log.Printf("skipping duplicate player: %s", player)
				continue
			}
			seen[player] = true
			currentGame = append(currentGame, player)
		}

		// Only add games with at least 2 players
		if len(currentGame) >= 2 {
			allGames = append(allGames, currentGame)
		}
	}

	// Apply windowing if specified
	var gamesToProcess [][]string
	if window > 0 && len(allGames) > window {
		gamesToProcess = allGames[len(allGames)-window:]
		log.Printf("Processing last %d games out of %d total games", window, len(allGames))
	} else {
		gamesToProcess = allGames
		if window > 0 {
			log.Printf("Processing all %d games (window size %d is larger than total games)", len(allGames), window)
		} else {
			log.Printf("Processing all %d games", len(allGames))
		}
	}

	// Process the selected games
	for _, currentGame := range gamesToProcess {
		log.Printf("comparing players: %s", currentGame)
		log.Printf("attempting to score game: %+v - %+v", scores, currentGame)

		err := ScoreGame(elo, scores, currentGame)
		if err != nil {
			return nil, fmt.Errorf("failed to score game: %v", err)
		}
	}

	// build final scores list
	finalScores := []finalScore{}
	for k, v := range scores {
		fs := finalScore{
			player:   k,
			eloScore: v,
		}
		finalScores = append(finalScores, fs)
	}

	// sort in descending order
	sort.Slice(finalScores, func(i int, j int) bool {
		return finalScores[i].eloScore > finalScores[j].eloScore
	})

	return finalScores, nil
}

// ScoreGame implements proper multiplayer Elo by comparing every player against every other player
// Players are assumed to be listed in finishing order (1st place first, last place last)
func ScoreGame(elo *elogo.Elo, scores map[string]int, game []string) error {
	numPlayers := len(game)
	if numPlayers < 2 {
		return fmt.Errorf("invalid game: need at least 2 players")
	}

	// Validate no duplicate players
	playerSet := make(map[string]bool)
	for _, player := range game {
		if playerSet[player] {
			return fmt.Errorf("invalid game: duplicate player %s", player)
		}
		playerSet[player] = true
	}

	// Initialize default scores for new players
	for _, player := range game {
		if _, exists := scores[player]; !exists {
			scores[player] = defaultStartingScore
		}
	}

	// Store original ratings to calculate all pairwise comparisons
	originalRatings := make(map[string]int)
	for _, player := range game {
		originalRatings[player] = scores[player]
	}

	// Calculate rating changes for each player based on all pairwise comparisons
	ratingChanges := make(map[string]int)
	for _, player := range game {
		ratingChanges[player] = 0
	}

	// Compare every player against every other player
	for i := 0; i < numPlayers; i++ {
		for j := 0; j < numPlayers; j++ {
			if i == j {
				continue // Don't compare player against themselves
			}

			playerA := game[i]
			playerB := game[j]

			ratingA := originalRatings[playerA]
			ratingB := originalRatings[playerB]

			// Player A's score against Player B
			// 1.0 if A finished better (lower index), 0.0 if A finished worse
			var score float64
			if i < j {
				score = 1.0 // Player A finished better than Player B
			} else {
				score = 0.0 // Player A finished worse than Player B
			}

			// Calculate rating change for this pairwise comparison
			delta := elo.RatingDelta(ratingA, ratingB, score)
			ratingChanges[playerA] += delta

			log.Printf("Pairwise: %s (pos %d, rating %d) vs %s (pos %d, rating %d) -> score %.1f, delta %d",
				playerA, i+1, ratingA, playerB, j+1, ratingB, score, delta)
		}
	}

	// Apply all rating changes
	for player, change := range ratingChanges {
		scores[player] = originalRatings[player] + change
		log.Printf("Player %s: %d -> %d (change: %+d)", player, originalRatings[player], scores[player], change)
	}

	return nil
}
