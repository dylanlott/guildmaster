package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/dylanlott/guildmaster/internal/analyzer"
)

func main() {
	path := flag.String("path", "./mtgscores.csv", "path to analyze with tracker")
	useTUI := flag.Bool("tui", false, "use terminal UI for displaying rankings")
	flag.Parse()

	if *useTUI {
		log.SetOutput(nil)
	}

	elo := analyzer.InitializeElo()
	scores := make(map[string]int)

	if err := analyzer.ProcessScores(*path, elo, scores); err != nil {
		log.Fatalf("Error processing scores: %v", err)
	}

	finalScores := analyzer.CalculateFinalScores(scores)

	if *useTUI {
		if err := analyzer.DisplayRankingsTUI(finalScores); err != nil {
			log.Fatalf("Error in TUI: %v", err)
		}
	} else {
		for i, v := range finalScores {
			fmt.Printf("%d --- %s --- %d\n", i+1, v.Player, v.EloScore)
		}
	}
}
