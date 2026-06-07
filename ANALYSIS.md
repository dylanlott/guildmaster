# Guildmaster — Codebase Analysis

## What It Is

A Go application for tracking Magic: The Gathering player rankings using **multiplayer pairwise Elo**. Functions as a mini league management system with three interfaces: a CLI tool, a terminal UI, and a web server.

---

## Architecture

### Entry Points

| Entry Point | Purpose |
|---|---|
| `main.go` | CLI tool — reads a CSV, prints rankings or launches TUI |
| `cmd/analyze/` | Separate CLI analyzer with its own TUI |
| `cmd/server/` | HTTP server with REST API + web UI |

### Internal Packages

- **`internal/scoring/store.go`** — Thread-safe in-memory Elo store. Exposes `ApplyDeltas`, `ReplaceAll`, `GetAll`, and `ScoreGame` for use by the server.
- **`internal/analyzer/analyzer.go`** — Core scoring logic: CSV parsing, pairwise Elo calculation, final score sorting.
- **`internal/server/handlers.go`** — HTTP handlers. Pulls live game data from Google Sheets via `sheets.go`, recomputes rankings on demand.

---

## Scoring System

Multiplayer pairwise Elo:

- **K = 40**, **D = 800** (wider than standard chess D=400 — more rating volatility per game)
- Every N-player game generates **N*(N-1)/2** pairwise matchups
- 1st place beats 2nd, 3rd, 4th...
- 2nd place loses to 1st but beats 3rd, 4th...
- And so on down the finishing order

Deltas are computed from a **pre-game snapshot** of all ratings, accumulated per player, then applied atomically. This prevents order-dependent updates and ensures deterministic results.

Default starting rating: **1500**

---

## Key Observations

### 1. Dual Data Sources
The CLI reads from a local `mtgscores.csv`; the web server pulls from **Google Sheets**. These two sources are not synced — they're entirely independent pipelines.

### 2. Code Duplication
The `ScoreGame` logic is nearly identical in three places:
- `main.go`
- `internal/analyzer/analyzer.go`
- `internal/scoring/store.go`

This should be consolidated into a single canonical implementation.

### 3. No Persistence
The server's scoring store is **in-memory only**. All computed ratings reset on restart. There's no database or file-based persistence layer.

### 4. Google Sheets Integration
`internal/server/sheets.go` is the bridge between the web server and live game data. Requires a Google service account or OAuth credentials configured via environment variables (see `.env.example`).

### 5. TUI
Built with the **Charmbracelet** stack (bubbletea + bubbles + lipgloss). Displays a ranked leaderboard with arrow key navigation. Implemented in both `tui.go` (root) and `internal/analyzer/tui.go` — another instance of duplication.

---

## Tech Stack

| Component | Library/Tool |
|---|---|
| Language | Go 1.25 |
| Elo engine | `github.com/kortemy/elo-go` |
| TUI | Charmbracelet (bubbletea, bubbles, lipgloss) |
| Sheets API | `google.golang.org/api` |
| Build | Make |
| Deploy | Docker / docker-compose |

---

## Potential Improvements

- **Consolidate `ScoreGame`** — single implementation in `internal/scoring`, used by both CLI and server
- **Add persistence** — SQLite or file-based store so ratings survive restarts
- **Unify data sources** — CLI and server should be able to use the same data source (Sheets or CSV)
- **Deduplicate TUI code** — `tui.go` and `internal/analyzer/tui.go` are redundant
- **Add more game history** — server currently only shows last 10 games; could be configurable
