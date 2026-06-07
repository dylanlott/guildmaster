package cli

import (
	"flag"
	"fmt"
	"io"
	"log"

	"github.com/dylanlott/guildmaster/internal/analyzer"
)

func RunAnalyze(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("guildmaster-analyze", flag.ContinueOnError)
	fs.SetOutput(stderr)

	path := fs.String("path", "./mtgscores.csv", "path to analyze with tracker")
	useTUI := fs.Bool("tui", false, "use terminal UI for displaying rankings")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *useTUI {
		log.SetOutput(io.Discard)
	} else {
		log.SetOutput(stderr)
	}

	scores := make(map[string]int)
	if err := analyzer.ProcessScores(*path, scores); err != nil {
		return fmt.Errorf("error processing scores: %w", err)
	}

	finalScores := analyzer.CalculateFinalScores(scores)
	if *useTUI {
		if err := analyzer.DisplayRankingsTUI(finalScores); err != nil {
			return fmt.Errorf("error in TUI: %w", err)
		}
		return nil
	}

	for i, v := range finalScores {
		fmt.Fprintf(stdout, "%d --- %s --- %d\n", i+1, v.Player, v.EloScore)
	}
	return nil
}
