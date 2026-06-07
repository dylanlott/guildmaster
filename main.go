package main

import (
	"log"
	"os"

	"github.com/dylanlott/guildmaster/internal/cli"
)

func main() {
	if err := cli.RunAnalyze(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		log.Fatalf("%v", err)
	}
}
