# Feature: Pod Game Recording Form

## Goal
Any pod member can record a game. Tracks placement + optional MTG/TCG metadata per player.

## Auth Change
Current: `/pods/:slug/submit` requires `GUILDMASTER_ADMIN_KEY` (API key auth).
New: any logged-in pod member can submit a game to their pod.
Keep API key auth on `POST /api/games` for programmatic access.

## Schema

### pods table — add format column
Format is set at the pod level (all games in a pod share the same format):
```sql
ALTER TABLE pods ADD COLUMN format TEXT NOT NULL DEFAULT 'commander';
-- Options: commander, standard, draft, sealed, pioneer, modern, legacy, custom
```
Pod owner sets format at creation time and can change it in pod settings.

### game_metadata (new table)
Optional per-game metadata (whole game context):
```sql
CREATE TABLE game_metadata (
    game_id     INTEGER PRIMARY KEY REFERENCES games(id) ON DELETE CASCADE,
    turn_count  INTEGER,          -- how many turns the game lasted
    notes       TEXT              -- freeform game notes
    -- format inherited from pod
);
```

### player_game_stats (new table)
Optional per-player per-game stats:
```sql
CREATE TABLE player_game_stats (
    game_id         INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    player_id       INTEGER NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    deck_name       TEXT,             -- what deck/commander they played
    starting_life   INTEGER DEFAULT 40,
    final_life      INTEGER,          -- life total when eliminated or game ended
    eliminations    INTEGER DEFAULT 0, -- how many other players they knocked out
    commander_damage_dealt INTEGER DEFAULT 0,  -- total cmd dmg dealt to opponents
    commander_damage_taken INTEGER DEFAULT 0,  -- total cmd dmg received
    turn_eliminated INTEGER,          -- which turn they were eliminated (NULL = survived)
    PRIMARY KEY (game_id, player_id)
);
```

## Form Design

### Step 1 — Players & Placements
- Dynamic player list: add players by username (pod members) or guest name
- Drag-to-reorder OR numbered rank dropdowns (1st, 2nd, 3rd...)
- Minimum 2 players
- Shows live Elo preview as you order players (JS, calls /api/scores or pod scores)

### Step 2 — Game Details (optional)
- Format selector: Commander (default), Standard, Draft, Sealed, Pioneer, Modern, Legacy, Custom
- Turn count (number input)
- Game notes (textarea)

### Step 3 — Per-Player Stats (optional, collapsible per player)
For each player:
- Deck/Commander name (text)
- Final life total (number, default 0 for eliminated)
- Eliminations (number, 0-N)
- Turn eliminated (number, blank = survived to end)
- Commander damage dealt / taken (numbers)

### UX
- Steps can be collapsed — Step 1 is required, Steps 2-3 are "Add more details (optional)"
- Submit after Step 1 is always valid — extra stats are bonus data
- Confirmation screen shows: placements, Elo changes per player, then confirm button
- After submit: redirect to the game detail page for the newly created game

## Routes
- `GET /pods/:slug/submit` → form (requires pod membership, logged in)
- `POST /pods/:slug/submit/create` → record game + stats, redirect to /pods/:slug/games/:id

## DB Methods needed
- `RecordPodGameWithStats(podID, playedAt, source, format, turnCount, notes string, rankings []string, playerStats []PlayerGameStat, snapshot map[string]int) (gameID int64, err error)`
- `GetGameWithStats(gameID int64) (*GameWithStats, error)` — extend existing GetGameByID
- `GetPodMemberNames(podID int64) ([]string, error)` — for player autocomplete

## Game Detail page update
Extend `/pods/:slug/games/:id` to show:
- Format, turn count, notes (if set)
- Per-player row: placement, deck name, Elo change, life total, eliminations, cmd dmg
