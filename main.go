package main

import (
	"encoding/csv"
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

type currentGame struct {
	winner string
	loser  string
}

type finalScore struct {
	player   string
	eloScore int
}

func main() {
	elo := elogo.NewElo()
	elo.D = 800
	elo.K = 40

	scores := map[string]int{}

	f, err := os.Open("mtgscores.csv")
	if err != nil {
		log.Fatalf("failed to open scores: %v", err)
	}
	reader := csv.NewReader(f)
	for {
		rec, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatalf("error processing record: %v", err)
		}

		players := rec[2:]
		currentGame := []string{}
		for _, player := range players {
			player = strings.TrimSpace(player)
			if player == "" {

				// score the game once we've assembled it.
				err := ScoreGame(elo, scores, currentGame)
				if err != nil {
					log.Fatalf("failed to score game: %v", err)
				}

				break
			}

			currentGame = append(currentGame, player)
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

	// print out final scores
	for i, v := range finalScores {
		fmt.Printf("%d --- %s --- %d\n", i, v.player, v.eloScore)
	}

	// log.Printf("player scores %v", scores)
}

// ScoreGame iterates over a game and updates scores in the playerMap accordingly
func ScoreGame(elo *elogo.Elo, scores map[string]int, game []string) error {
	numPlayers := len(game)
	if numPlayers < 2 {
		return fmt.Errorf("invalid game")
	}

	for place := range game {
		if place == numPlayers-1 {
			// last place
			break
		}

		// get player names
		playerA := game[place]
		playerB := game[place+1]

		// get player scores from our score map
		_, ok := scores[playerA]
		if !ok {
			scores[playerA] = defaultStartingScore
		}

		_, ok = scores[playerB]
		if !ok {
			scores[playerB] = defaultStartingScore
		}

		// get ranks after we've assured defaults
		rankA := scores[playerA]
		rankB := scores[playerB]

		// check existence of player in map and set default score if they don't exist
		// log.Printf("comparing ranks %s - %d to %s - %d", playerA, rankA, playerB, rankB)

		// Results for A in the outcome of A defeats B
		score := 1                                             // Use 1 in case A wins, 0 in case B wins, 0.5 in case of a draw
		delta := elo.RatingDelta(rankA, rankB, float64(score)) // 20
		// log.Printf("rating delta between %s and %s: %d", playerA, playerB, delta)

		updatedRating := elo.Rating(rankA, rankB, float64(score)) // 1520

		scores[playerA] = updatedRating
		// log.Printf("updated %s score: %d", playerA, scores[playerA])

		scores[playerB] = scores[playerB] - delta
		// log.Printf("updated %s score: %d", playerB, scores[playerB])
	}
	return nil
}
