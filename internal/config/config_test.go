package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("SPREADSHEET_ID", "")
	t.Setenv("SCOREBOARD_API_KEY", "")
	t.Setenv("GUILDMASTER_ADMIN_KEY", "")
	t.Setenv("DATABASE_PATH", "")
	t.Setenv("PORT", "")

	cfg := Load()
	if cfg.DatabasePath != "./guildmaster.db" {
		t.Fatalf("expected default database path, got %q", cfg.DatabasePath)
	}
	if cfg.Port != "8080" {
		t.Fatalf("expected default port, got %q", cfg.Port)
	}
	if cfg.ListenAddr() != ":8080" {
		t.Fatalf("expected listen addr :8080, got %q", cfg.ListenAddr())
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("SPREADSHEET_ID", "sheet123")
	t.Setenv("SCOREBOARD_API_KEY", "apikey")
	t.Setenv("GUILDMASTER_ADMIN_KEY", "admin")
	t.Setenv("DATABASE_PATH", "./data/guildmaster.db")
	t.Setenv("PORT", "9090")

	cfg := Load()
	if cfg.SpreadsheetID != "sheet123" {
		t.Fatalf("unexpected spreadsheet id: %q", cfg.SpreadsheetID)
	}
	if cfg.ScoreboardAPIKey != "apikey" {
		t.Fatalf("unexpected api key: %q", cfg.ScoreboardAPIKey)
	}
	if cfg.GuildmasterAdminKey != "admin" {
		t.Fatalf("unexpected admin key: %q", cfg.GuildmasterAdminKey)
	}
	if cfg.DatabasePath != "./data/guildmaster.db" {
		t.Fatalf("unexpected database path: %q", cfg.DatabasePath)
	}
	if cfg.ListenAddr() != ":9090" {
		t.Fatalf("unexpected listen addr: %q", cfg.ListenAddr())
	}
}

func TestListenAddrPassthroughColon(t *testing.T) {
	cfg := Config{Port: ":7777"}
	if cfg.ListenAddr() != ":7777" {
		t.Fatalf("expected passthrough, got %q", cfg.ListenAddr())
	}
}

func TestSheetsConfigured(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{
			name: "both set",
			cfg: Config{
				SpreadsheetID:    "sheet123",
				ScoreboardAPIKey: "key123",
			},
			want: true,
		},
		{
			name: "missing spreadsheet id",
			cfg: Config{
				ScoreboardAPIKey: "key123",
			},
			want: false,
		},
		{
			name: "missing api key",
			cfg: Config{
				SpreadsheetID: "sheet123",
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.SheetsConfigured(); got != tc.want {
				t.Fatalf("SheetsConfigured() = %v, want %v", got, tc.want)
			}
		})
	}
}
