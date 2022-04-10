package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"

	elogo "github.com/kortemy/elo-go"
)

type currentGame struct {
	winner string
	loser  string
}

func main() {
	elo := elogo.NewElo()
	scores := map[string]int{}

	f, err := os.Open("mtgscores.csv")
	if err != nil {
		log.Fatalf("failed to open scores: %v", err)
	}
	reader := csv.NewReader(f)
	for {
		rec, err := reader.Read()
		if err != nil {
			log.Printf("error processing record: %v", err)
			break
		}
		if err == io.EOF {
			log.Printf("finished scanning csv records")
			break
		}

		// each record is a game so we need to run all of the scores for the game.
		// fmt.Printf("rec: %v\n", rec)
		// date := rec[0]

		players := rec[2:]

		currentGame := []string{}
		for _, player := range players {
			if player == "" {
				// end of players, end of game def
				log.Printf("game: %v", currentGame)

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
	log.Printf("player scores %v", scores)
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

		// get player scorse
		_, ok := scores[playerA]
		if !ok {
			scores[playerA] = 1500
		}

		_, ok = scores[playerB]
		if !ok {
			scores[playerB] = 1500
		}

		// get ranks after we've assured defaults
		rankA := scores[playerA]
		rankB := scores[playerB]

		// check existence of player in map and set default score if they don't exist
		log.Printf("comparing ranks %s - %d to %s - %d", playerA, rankA, playerB, rankB)

		// Results for A in the outcome of A defeats B
		score := 1                                             // Use 1 in case A wins, 0 in case B wins, 0.5 in case of a draw
		delta := elo.RatingDelta(rankA, rankB, float64(score)) // 20
		log.Printf("rating delta between %s and %s: %d", playerA, playerB, delta)

		updatedRating := elo.Rating(rankA, rankB, float64(score)) // 1520

		scores[playerA] = updatedRating
		log.Printf("updated %s score: %d", playerA, scores[playerA])

		scores[playerB] = scores[playerB] - delta
		log.Printf("updated %s score: %d", playerB, scores[playerB])

	}

	return nil
}
