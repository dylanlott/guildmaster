package config

import (
	"os"
	"strings"
)

const (
	defaultDatabasePath = "./guildmaster.db"
	defaultPort         = "8080"
)

type Config struct {
	SpreadsheetID       string
	ScoreboardAPIKey    string
	GuildmasterAdminKey string
	DatabasePath        string
	Port                string
}

func Load() Config {
	cfg := Config{
		SpreadsheetID:       strings.TrimSpace(os.Getenv("SPREADSHEET_ID")),
		ScoreboardAPIKey:    strings.TrimSpace(os.Getenv("SCOREBOARD_API_KEY")),
		GuildmasterAdminKey: strings.TrimSpace(os.Getenv("GUILDMASTER_ADMIN_KEY")),
		DatabasePath:        strings.TrimSpace(os.Getenv("DATABASE_PATH")),
		Port:                strings.TrimSpace(os.Getenv("PORT")),
	}

	if cfg.DatabasePath == "" {
		cfg.DatabasePath = defaultDatabasePath
	}
	if cfg.Port == "" {
		cfg.Port = defaultPort
	}
	return cfg
}

func (c Config) ListenAddr() string {
	if strings.HasPrefix(c.Port, ":") {
		return c.Port
	}
	return ":" + c.Port
}

func (c Config) SheetsConfigured() bool {
	return c.SpreadsheetID != "" && c.ScoreboardAPIKey != ""
}
