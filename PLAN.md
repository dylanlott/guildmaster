# Guildmaster — Web App Improvement Plan

## Current State

The web server exists but is minimal:
- Server-rendered Go template (`landing.tmpl`) with dark theme
- Pulls live data from a hardcoded Google Sheets spreadsheet
- In-memory only — scores reset on restart
- No auth, no game submission UI beyond the raw `assets/index.html` dev tool
- No player profiles, game history detail, or stats beyond raw Elo score
- Duplicate scoring logic across three files
- Two-headed giant games are silently skipped

---

## Goals

Turn Guildmaster into a proper, self-contained web app:
- Real persistence (survive restarts, support history queries)
- Clean public-facing UI with player profiles and game history
- Admin-protected game submission
- Keep the Sheets integration as an optional import source
- Consolidate and harden the scoring engine

---

## Phase 1 — Foundation (Persistence + Code Cleanup)

### 1.1 — Consolidate Scoring Engine

**Problem:** `ScoreGame` is duplicated in `main.go`, `internal/analyzer/analyzer.go`, and `internal/scoring/store.go`.

**Action:**
- Make `internal/scoring` the single source of truth for all Elo logic
- Remove duplicates from `main.go` and `internal/analyzer/analyzer.go`
- Update `internal/analyzer` to import from `internal/scoring`

### 1.2 — Add a Persistence Layer

**Problem:** All scores live in memory. Server restart = data loss.

**Action:**
- Add **SQLite** via `modernc.org/sqlite` (pure Go, no CGo)
- Schema:
  ```sql
  CREATE TABLE players (id INTEGER PRIMARY KEY, name TEXT UNIQUE, elo INTEGER DEFAULT 1500);
  CREATE TABLE games (id INTEGER PRIMARY KEY, played_at DATETIME, source TEXT);
  CREATE TABLE game_players (game_id INTEGER, player_id INTEGER, placement INTEGER);
  ```
- Wrap in a `db.Store` that implements the same interface as the current `scoring.Store`
- On startup: load current Elo from DB into memory; write through to DB on each game submission

### 1.3 — Sheets Import Command

**Problem:** Sheets is the existing data source; we can't discard it.

**Action:**
- Add a CLI command `make import` / `go run ./cmd/import` that:
  1. Fetches all game rows from Sheets
  2. Replays them chronologically through the scoring engine
  3. Writes the resulting game records + ratings into SQLite
- This becomes a one-time migration and an optional ongoing sync

---

## Phase 2 — Web API

### 2.1 — RESTful Routes

Expand the API surface:

| Method | Route | Description |
|---|---|---|
| `GET` | `/api/scores` | All player scores, sorted desc |
| `GET` | `/api/players/:name` | Player profile + game history |
| `GET` | `/api/games` | Paginated game history |
| `GET` | `/api/games/:id` | Single game detail with placements |
| `POST` | `/api/games` | Submit a new game (auth required) |
| `POST` | `/api/refresh` | Re-sync from Sheets (auth required) |

### 2.2 — Authentication

**Action:**
- Simple API key auth for write endpoints (`POST /api/games`, `POST /api/refresh`)
- Key loaded from env var `GUILDMASTER_ADMIN_KEY`
- Middleware checks `Authorization: Bearer <key>` header
- No need for full user auth — this is a small group tool

---

## Phase 3 — UI Overhaul

The current `landing.tmpl` is a good start but needs more pages and interactivity.

### 3.1 — Pages

| Page | Route | Description |
|---|---|---|
| Leaderboard | `/` | Full ranked scoreboard, filterable |
| Player Profile | `/players/:name` | Elo history chart, win/loss record, recent games |
| Game History | `/games` | Paginated list of all games |
| Game Detail | `/games/:id` | Who played, placements, Elo changes per player |
| Submit Game | `/submit` | Admin-gated form to enter results |

### 3.2 — Leaderboard Enhancements

Current board shows rank + name + score. Add:
- **Δ** column: Elo change from last game
- **Games played** count
- **Win rate** (1st place finishes / total games)
- Sortable columns (client-side)

### 3.3 — Player Profile Page

- Elo trend chart over time (use a lightweight lib like Chart.js or just SVG sparklines)
- Recent game history table (placement, opponents, Elo change)
- Head-to-head record vs other players

### 3.4 — Submit Game Form

- Ordered list with drag-to-reorder (or simple numbered dropdowns)
- Preview of projected Elo changes before submitting
- Confirmation dialog

### 3.5 — Tech Choice for Frontend

Keep it server-rendered Go templates + vanilla JS (no build step, minimal deps). Use HTMX for partial page updates if interactivity grows. Avoid a full JS framework unless the complexity demands it.

---

## Phase 4 — Ops & Quality

### 4.1 — Tests

- Unit tests for scoring engine (already has `store_test.go` — expand coverage)
- Integration tests for HTTP handlers with an in-memory SQLite DB
- Test the Sheets parser against fixture data

### 4.2 — Config

Move all hardcoded values to env vars / a config struct:
- `SPREADSHEET_ID` (currently hardcoded in `sheets.go`)
- `SCOREBOARD_API_KEY`
- `GUILDMASTER_ADMIN_KEY`
- `DATABASE_PATH` (default: `./guildmaster.db`)
- `PORT` (default: `8080`)

### 4.3 — Docker

Update `Dockerfile` to:
- Use multi-stage build
- Mount a volume for the SQLite DB file
- Accept all config via env vars

### 4.4 — CLI Cleanup

- Remove `main.go` root-level binary or keep it as a thin wrapper that uses `internal/scoring`
- Consolidate `cmd/analyze` and the root CLI — they do the same thing
- Keep `cmd/server` as the canonical server entry point

---

## Execution Order

```
Phase 1.1 → 1.2 → 1.3   (foundation, unblocks everything)
Phase 2.1 → 2.2          (API, needed by UI)
Phase 3.1 → 3.2 → 3.3   (UI, iterative)
Phase 4 throughout        (tests/config as you go)
```

---

## Out of Scope (For Now)

- User accounts / login
- Multiplayer game types beyond standard Elo (Commander variants, etc.)
- Mobile app
- Two-headed giant support (currently skipped — revisit later)
- Real-time updates (WebSocket leaderboard)
