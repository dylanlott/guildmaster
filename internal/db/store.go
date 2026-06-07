package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/dylanlott/guildmaster/internal/scoring"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type migration struct {
	version    int
	statements []string
}

type PlayerScore struct {
	Name string `json:"name"`
	Elo  int    `json:"elo"`
}

type GamePlacement struct {
	Placement int    `json:"placement"`
	Player    string `json:"player"`
}

type GameRecord struct {
	ID         int64           `json:"id"`
	PlayedAt   time.Time       `json:"played_at"`
	Source     string          `json:"source"`
	Placements []GamePlacement `json:"placements"`
}

type PlayerGameRecord struct {
	GameID     int64           `json:"game_id"`
	PlayedAt   time.Time       `json:"played_at"`
	Source     string          `json:"source"`
	Placement  int             `json:"placement"`
	Placements []GamePlacement `json:"placements"`
}

type PlayerProfile struct {
	Name  string             `json:"name"`
	Elo   int                `json:"elo"`
	Games []PlayerGameRecord `json:"games"`
}

type PlayerStats struct {
	TotalGames   int     `json:"total_games"`
	Wins         int     `json:"wins"`
	WinRate      float64 `json:"win_rate"`
	AvgPlacement float64 `json:"avg_placement"`
}

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	DisplayName  string    `json:"display_name"`
	Role         string    `json:"role"`
	PasswordHash string    `json:"-"`
	GlobalElo    int       `json:"global_elo"`
	CreatedAt    time.Time `json:"created_at"`
}

type Pod struct {
	ID          int64     `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Format      string    `json:"format"`
	OwnerID     int64     `json:"owner_id"`
	Public      bool      `json:"public"`
	CreatedAt   time.Time `json:"created_at"`
}

type PlayerGameStat struct {
	Player               string
	DeckName             string
	StartingLife         int
	FinalLife            *int
	Eliminations         int
	CommanderDamageDealt int
	CommanderDamageTaken int
	TurnEliminated       *int
}

type GamePlayerWithStats struct {
	Placement            int
	Player               string
	DeckName             string
	StartingLife         int
	FinalLife            *int
	Eliminations         int
	CommanderDamageDealt int
	CommanderDamageTaken int
	TurnEliminated       *int
}

type GameWithStats struct {
	ID         int64
	PodID      int64
	PlayedAt   time.Time
	Source     string
	PodFormat  string
	TurnCount  *int
	Notes      *string
	Placements []GamePlayerWithStats
}

type PodMember struct {
	PodID    int64     `json:"pod_id"`
	UserID   int64     `json:"user_id"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

type PodLeaderboardRow struct {
	Name    string  `json:"name"`
	Elo     int     `json:"elo"`
	Games   int     `json:"games"`
	Wins    int     `json:"wins"`
	WinRate float64 `json:"win_rate"`
}

var schemaMigrations = []migration{
	{
		version: 1,
		statements: []string{
			`CREATE TABLE IF NOT EXISTS players (id INTEGER PRIMARY KEY, name TEXT UNIQUE, elo INTEGER NOT NULL DEFAULT 1500)`,
			`CREATE TABLE IF NOT EXISTS games (id INTEGER PRIMARY KEY, played_at DATETIME NOT NULL, source TEXT NOT NULL)`,
			`CREATE TABLE IF NOT EXISTS game_players (game_id INTEGER NOT NULL, player_id INTEGER NOT NULL, placement INTEGER NOT NULL)`,
		},
	},
	{
		version: 2,
		statements: []string{
			`CREATE INDEX IF NOT EXISTS idx_players_elo_name ON players (elo DESC, name ASC)`,
			`CREATE INDEX IF NOT EXISTS idx_games_played_at_id ON games (played_at DESC, id DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_game_players_game_id_placement ON game_players (game_id, placement)`,
			`CREATE INDEX IF NOT EXISTS idx_game_players_player_id_game_id ON game_players (player_id, game_id DESC)`,
		},
	},
	{
		version: 3,
		statements: []string{
			`CREATE TABLE users (
				id            INTEGER PRIMARY KEY,
				username      TEXT UNIQUE NOT NULL,
				display_name  TEXT NOT NULL,
				password_hash TEXT NOT NULL,
				global_elo    INTEGER NOT NULL DEFAULT 1500,
				created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE sessions (
				token      TEXT PRIMARY KEY,
				user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				expires_at DATETIME NOT NULL,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		},
	},
	{
		version: 4,
		statements: []string{
			`ALTER TABLE players ADD COLUMN user_id INTEGER REFERENCES users(id)`,
		},
	},
	{
		version: 5,
		statements: []string{
			`CREATE TABLE pods (
				id          INTEGER PRIMARY KEY,
				slug        TEXT UNIQUE NOT NULL,
				name        TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				owner_id    INTEGER NOT NULL REFERENCES users(id),
				public      INTEGER NOT NULL DEFAULT 1,
				created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE pod_members (
				pod_id    INTEGER NOT NULL REFERENCES pods(id) ON DELETE CASCADE,
				user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				role      TEXT NOT NULL DEFAULT 'member',
				joined_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (pod_id, user_id)
			)`,
			`CREATE TABLE pod_elos (
				pod_id    INTEGER NOT NULL REFERENCES pods(id) ON DELETE CASCADE,
				player_id INTEGER NOT NULL REFERENCES players(id) ON DELETE CASCADE,
				elo       INTEGER NOT NULL DEFAULT 1500,
				updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (pod_id, player_id)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_pods_slug ON pods(slug)`,
			`CREATE INDEX IF NOT EXISTS idx_pod_members_user_id ON pod_members(user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_pod_elos_pod_id ON pod_elos(pod_id)`,
			`ALTER TABLE games ADD COLUMN pod_id INTEGER REFERENCES pods(id)`,
		},
	},
	{
		version: 6,
		statements: []string{
			`ALTER TABLE pods ADD COLUMN format TEXT NOT NULL DEFAULT 'commander'`,
		},
	},
	{
		version: 7,
		statements: []string{
			`CREATE TABLE game_metadata (
				game_id     INTEGER PRIMARY KEY REFERENCES games(id) ON DELETE CASCADE,
				turn_count  INTEGER,
				notes       TEXT
			)`,
			`CREATE TABLE player_game_stats (
				game_id                  INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
				player_id                INTEGER NOT NULL REFERENCES players(id) ON DELETE CASCADE,
				deck_name                TEXT,
				starting_life            INTEGER DEFAULT 40,
				final_life               INTEGER,
				eliminations             INTEGER DEFAULT 0,
				commander_damage_dealt   INTEGER DEFAULT 0,
				commander_damage_taken   INTEGER DEFAULT 0,
				turn_eliminated          INTEGER,
				PRIMARY KEY (game_id, player_id)
			)`,
		},
	},
	{
		version: 8,
		statements: []string{
			`ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'member'`,
			`UPDATE users
			SET role = 'owner'
			WHERE id = (
				SELECT id FROM users ORDER BY created_at ASC, id ASC LIMIT 1
			)
			AND NOT EXISTS (
				SELECT 1 FROM users WHERE role IN ('owner', 'admin')
			)`,
		},
	},
}

func NewStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	store := &Store{db: db}
	if err := store.configureConnection(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.runMigrations(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) configureConnection() error {
	if _, err := s.db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		return fmt.Errorf("enable wal mode: %w", err)
	}
	if _, err := s.db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		return fmt.Errorf("enable foreign key constraints: %w", err)
	}
	return nil
}

func (s *Store) runMigrations() error {
	if _, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY,
	applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
)`); err != nil {
		return fmt.Errorf("initialize schema migrations table: %w", err)
	}

	applied := make(map[int]struct{})
	rows, err := s.db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("list applied migrations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("scan migration version: %w", err)
		}
		applied[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate migration versions: %w", err)
	}

	for _, m := range schemaMigrations {
		if _, ok := applied[m.version]; ok {
			continue
		}
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d transaction: %w", m.version, err)
		}

		for _, stmt := range m.statements {
			if _, err := tx.Exec(stmt); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("apply migration %d: %w", m.version, err)
			}
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, m.version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.version, err)
		}
	}

	return nil
}

func (s *Store) LoadScores() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT name, elo FROM players`)
	if err != nil {
		return nil, fmt.Errorf("load scores query: %w", err)
	}
	defer rows.Close()

	scores := make(map[string]int)
	for rows.Next() {
		var name string
		var elo int
		if err := rows.Scan(&name, &elo); err != nil {
			return nil, fmt.Errorf("scan score row: %w", err)
		}
		scores[name] = elo
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate score rows: %w", err)
	}
	return scores, nil
}

func (s *Store) Reset() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin reset transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM game_players`); err != nil {
		return fmt.Errorf("clear game_players: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM games`); err != nil {
		return fmt.Errorf("clear games: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM players`); err != nil {
		return fmt.Errorf("clear players: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reset transaction: %w", err)
	}
	return nil
}

func (s *Store) RecordGame(playedAt time.Time, source string, rankings []string, snapshot map[string]int) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin game transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(`INSERT INTO games (played_at, source) VALUES (?, ?)`, playedAt.UTC(), source)
	if err != nil {
		return 0, fmt.Errorf("insert game: %w", err)
	}
	gameID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read inserted game id: %w", err)
	}

	for i, name := range rankings {
		playerID, err := getOrCreatePlayerID(tx, name)
		if err != nil {
			return 0, err
		}
		if _, err := tx.Exec(`INSERT INTO game_players (game_id, player_id, placement) VALUES (?, ?, ?)`, gameID, playerID, i+1); err != nil {
			return 0, fmt.Errorf("insert game player row: %w", err)
		}

		elo, ok := snapshot[name]
		if !ok {
			elo = scoring.DefaultStartingScore
		}
		if _, err := tx.Exec(`UPDATE players SET elo = ? WHERE id = ?`, elo, playerID); err != nil {
			return 0, fmt.Errorf("update player elo: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit game transaction: %w", err)
	}
	return gameID, nil
}

func (s *Store) GetSortedScores() ([]PlayerScore, error) {
	rows, err := s.db.Query(`SELECT name, elo FROM players ORDER BY elo DESC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list scores query: %w", err)
	}
	defer rows.Close()

	scores := make([]PlayerScore, 0)
	for rows.Next() {
		var p PlayerScore
		if err := rows.Scan(&p.Name, &p.Elo); err != nil {
			return nil, fmt.Errorf("scan score row: %w", err)
		}
		scores = append(scores, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate score rows: %w", err)
	}
	return scores, nil
}

func (s *Store) ListGames(page, limit int) ([]GameRecord, int, error) {
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM games`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count games: %w", err)
	}

	offset := (page - 1) * limit
	rows, err := s.db.Query(`SELECT id, played_at, source FROM games ORDER BY played_at DESC, id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list games query: %w", err)
	}
	defer rows.Close()

	games := make([]GameRecord, 0, limit)
	for rows.Next() {
		var g GameRecord
		if err := rows.Scan(&g.ID, &g.PlayedAt, &g.Source); err != nil {
			return nil, 0, fmt.Errorf("scan game row: %w", err)
		}
		placements, err := s.loadGamePlacements(g.ID)
		if err != nil {
			return nil, 0, err
		}
		g.Placements = placements
		games = append(games, g)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate game rows: %w", err)
	}
	return games, total, nil
}

func (s *Store) ListPodGames(podID int64, page, limit int) ([]GameRecord, int, error) {
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM games WHERE pod_id = ?`, podID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count pod games: %w", err)
	}

	offset := (page - 1) * limit
	rows, err := s.db.Query(`
SELECT id, played_at, source
FROM games
WHERE pod_id = ?
ORDER BY played_at DESC, id DESC
LIMIT ? OFFSET ?
`, podID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list pod games query: %w", err)
	}
	defer rows.Close()

	games := make([]GameRecord, 0, limit)
	for rows.Next() {
		var g GameRecord
		if err := rows.Scan(&g.ID, &g.PlayedAt, &g.Source); err != nil {
			return nil, 0, fmt.Errorf("scan pod game row: %w", err)
		}
		placements, err := s.loadGamePlacements(g.ID)
		if err != nil {
			return nil, 0, err
		}
		g.Placements = placements
		games = append(games, g)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate pod game rows: %w", err)
	}
	return games, total, nil
}

func (s *Store) GetGameByID(gameID int64) (*GameRecord, error) {
	var g GameRecord
	err := s.db.QueryRow(`SELECT id, played_at, source FROM games WHERE id = ?`, gameID).Scan(&g.ID, &g.PlayedAt, &g.Source)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get game query: %w", err)
	}

	placements, err := s.loadGamePlacements(gameID)
	if err != nil {
		return nil, err
	}
	g.Placements = placements
	return &g, nil
}

func (s *Store) GetPodGameByID(podID, gameID int64) (*GameRecord, error) {
	var g GameRecord
	err := s.db.QueryRow(`
SELECT id, played_at, source
FROM games
WHERE id = ? AND pod_id = ?
`, gameID, podID).Scan(&g.ID, &g.PlayedAt, &g.Source)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get pod game query: %w", err)
	}

	placements, err := s.loadGamePlacements(gameID)
	if err != nil {
		return nil, err
	}
	g.Placements = placements
	return &g, nil
}

func (s *Store) ListAllGamesChronological() ([]GameRecord, error) {
	rows, err := s.db.Query(`SELECT id, played_at, source FROM games ORDER BY played_at ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list chronological games query: %w", err)
	}
	defer rows.Close()

	games := make([]GameRecord, 0)
	for rows.Next() {
		var g GameRecord
		if err := rows.Scan(&g.ID, &g.PlayedAt, &g.Source); err != nil {
			return nil, fmt.Errorf("scan chronological game row: %w", err)
		}
		placements, err := s.loadGamePlacements(g.ID)
		if err != nil {
			return nil, err
		}
		g.Placements = placements
		games = append(games, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chronological game rows: %w", err)
	}

	return games, nil
}

func (s *Store) ListPodGamesChronological(podID int64) ([]GameRecord, error) {
	rows, err := s.db.Query(`
SELECT id, played_at, source
FROM games
WHERE pod_id = ?
ORDER BY played_at ASC, id ASC
`, podID)
	if err != nil {
		return nil, fmt.Errorf("list chronological pod games query: %w", err)
	}
	defer rows.Close()

	games := make([]GameRecord, 0)
	for rows.Next() {
		var g GameRecord
		if err := rows.Scan(&g.ID, &g.PlayedAt, &g.Source); err != nil {
			return nil, fmt.Errorf("scan chronological pod game row: %w", err)
		}
		placements, err := s.loadGamePlacements(g.ID)
		if err != nil {
			return nil, err
		}
		g.Placements = placements
		games = append(games, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chronological pod game rows: %w", err)
	}

	return games, nil
}

func (s *Store) GetPlayerProfile(name string) (*PlayerProfile, error) {
	var (
		playerID int64
		profile  PlayerProfile
	)
	err := s.db.QueryRow(`SELECT id, name, elo FROM players WHERE name = ?`, name).Scan(&playerID, &profile.Name, &profile.Elo)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get player query: %w", err)
	}

	rows, err := s.db.Query(`
SELECT g.id, g.played_at, g.source, gp.placement
FROM game_players gp
JOIN games g ON g.id = gp.game_id
WHERE gp.player_id = ?
ORDER BY g.played_at DESC, g.id DESC
`, playerID)
	if err != nil {
		return nil, fmt.Errorf("player games query: %w", err)
	}
	defer rows.Close()

	profile.Games = make([]PlayerGameRecord, 0)
	for rows.Next() {
		var rec PlayerGameRecord
		if err := rows.Scan(&rec.GameID, &rec.PlayedAt, &rec.Source, &rec.Placement); err != nil {
			return nil, fmt.Errorf("scan player game row: %w", err)
		}
		placements, err := s.loadGamePlacements(rec.GameID)
		if err != nil {
			return nil, err
		}
		rec.Placements = placements
		profile.Games = append(profile.Games, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate player game rows: %w", err)
	}

	return &profile, nil
}

func (s *Store) GetPlayerStats() (map[string]PlayerStats, error) {
	rows, err := s.db.Query(`
SELECT
	p.name,
	COUNT(gp.game_id) AS total_games,
	COALESCE(SUM(CASE WHEN gp.placement = 1 THEN 1 ELSE 0 END), 0) AS wins,
	COALESCE(AVG(CAST(gp.placement AS REAL)), 0) AS avg_placement
FROM players p
LEFT JOIN game_players gp ON gp.player_id = p.id
GROUP BY p.id, p.name
`)
	if err != nil {
		return nil, fmt.Errorf("get player stats query: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]PlayerStats)
	for rows.Next() {
		var (
			name         string
			totalGames   int
			wins         int
			avgPlacement float64
		)
		if err := rows.Scan(&name, &totalGames, &wins, &avgPlacement); err != nil {
			return nil, fmt.Errorf("scan player stats row: %w", err)
		}
		winRate := 0.0
		if totalGames > 0 {
			winRate = (float64(wins) / float64(totalGames)) * 100.0
		}
		stats[name] = PlayerStats{
			TotalGames:   totalGames,
			Wins:         wins,
			WinRate:      winRate,
			AvgPlacement: avgPlacement,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate player stats rows: %w", err)
	}
	return stats, nil
}

func (s *Store) CreateUser(username, displayName, passwordHash string) (*User, error) {
	res, err := s.db.Exec(`
INSERT INTO users (username, display_name, password_hash, role)
VALUES (
	?,
	?,
	?,
	CASE
		WHEN EXISTS (SELECT 1 FROM users WHERE role IN ('owner', 'admin')) THEN 'member'
		ELSE 'owner'
	END
)
`, username, displayName, passwordHash)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("read inserted user id: %w", err)
	}
	return s.GetUserByID(id)
}

func (s *Store) GetUserByUsername(username string) (*User, error) {
	var user User
	err := s.db.QueryRow(`
SELECT id, username, display_name, role, password_hash, global_elo, created_at
FROM users
WHERE username = ?
`, username).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Role, &user.PasswordHash, &user.GlobalElo, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	return &user, nil
}

func (s *Store) GetUserByID(id int64) (*User, error) {
	var user User
	err := s.db.QueryRow(`
SELECT id, username, display_name, role, password_hash, global_elo, created_at
FROM users
WHERE id = ?
`, id).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Role, &user.PasswordHash, &user.GlobalElo, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &user, nil
}

func (s *Store) CreateSession(userID int64, token string, expiresAt time.Time) error {
	if _, err := s.db.Exec(`
INSERT INTO sessions (token, user_id, expires_at)
VALUES (?, ?, ?)
`, token, userID, expiresAt.UTC()); err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

func (s *Store) GetSessionUser(token string) (*User, error) {
	var user User
	err := s.db.QueryRow(`
SELECT u.id, u.username, u.display_name, u.role, u.password_hash, u.global_elo, u.created_at
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token = ? AND s.expires_at > ?
`, token, time.Now().UTC()).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Role, &user.PasswordHash, &user.GlobalElo, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session user: %w", err)
	}
	return &user, nil
}

func (s *Store) DeleteSession(token string) error {
	if _, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *Store) UpdateUserGlobalElo(userID int64, elo int) error {
	if _, err := s.db.Exec(`UPDATE users SET global_elo = ? WHERE id = ?`, elo, userID); err != nil {
		return fmt.Errorf("update user global elo: %w", err)
	}
	return nil
}

func (s *Store) CountPodMembers(podID int64) (int, error) {
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pod_members WHERE pod_id = ?`, podID).Scan(&total); err != nil {
		return 0, fmt.Errorf("count pod members: %w", err)
	}
	return total, nil
}

func (s *Store) CreatePod(ownerID int64, slug, name, description, format string, public bool) (*Pod, error) {
	if format == "" {
		format = "commander"
	}
	publicInt := 0
	if public {
		publicInt = 1
	}
	res, err := s.db.Exec(`
INSERT INTO pods (slug, name, description, format, owner_id, public)
VALUES (?, ?, ?, ?, ?, ?)
`, slug, name, description, format, ownerID, publicInt)
	if err != nil {
		return nil, fmt.Errorf("insert pod: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("read inserted pod id: %w", err)
	}
	return s.GetPodByID(id)
}

func (s *Store) GetPodBySlug(slug string) (*Pod, error) {
	var (
		pod       Pod
		publicInt int
	)
	err := s.db.QueryRow(`
SELECT id, slug, name, description, format, owner_id, public, created_at
FROM pods
WHERE slug = ?
`, slug).Scan(&pod.ID, &pod.Slug, &pod.Name, &pod.Description, &pod.Format, &pod.OwnerID, &publicInt, &pod.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get pod by slug: %w", err)
	}
	pod.Public = publicInt == 1
	return &pod, nil
}

func (s *Store) GetPodByID(id int64) (*Pod, error) {
	var (
		pod       Pod
		publicInt int
	)
	err := s.db.QueryRow(`
SELECT id, slug, name, description, format, owner_id, public, created_at
FROM pods
WHERE id = ?
`, id).Scan(&pod.ID, &pod.Slug, &pod.Name, &pod.Description, &pod.Format, &pod.OwnerID, &publicInt, &pod.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get pod by id: %w", err)
	}
	pod.Public = publicInt == 1
	return &pod, nil
}

func (s *Store) ListUserPods(userID int64) ([]*Pod, error) {
	rows, err := s.db.Query(`
SELECT p.id, p.slug, p.name, p.description, p.format, p.owner_id, p.public, p.created_at
FROM pod_members pm
JOIN pods p ON p.id = pm.pod_id
WHERE pm.user_id = ?
ORDER BY p.name ASC, p.id ASC
`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user pods: %w", err)
	}
	defer rows.Close()

	pods := make([]*Pod, 0)
	for rows.Next() {
		var (
			pod       Pod
			publicInt int
		)
		if err := rows.Scan(&pod.ID, &pod.Slug, &pod.Name, &pod.Description, &pod.Format, &pod.OwnerID, &publicInt, &pod.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user pod row: %w", err)
		}
		pod.Public = publicInt == 1
		pods = append(pods, &pod)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user pods: %w", err)
	}
	return pods, nil
}

func (s *Store) ListPublicPods() ([]*Pod, error) {
	rows, err := s.db.Query(`
SELECT id, slug, name, description, format, owner_id, public, created_at
FROM pods
WHERE public = 1
ORDER BY name ASC, id ASC
`)
	if err != nil {
		return nil, fmt.Errorf("list public pods: %w", err)
	}
	defer rows.Close()

	pods := make([]*Pod, 0)
	for rows.Next() {
		var (
			pod       Pod
			publicInt int
		)
		if err := rows.Scan(&pod.ID, &pod.Slug, &pod.Name, &pod.Description, &pod.Format, &pod.OwnerID, &publicInt, &pod.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan public pod row: %w", err)
		}
		pod.Public = publicInt == 1
		pods = append(pods, &pod)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate public pods: %w", err)
	}
	return pods, nil
}

func (s *Store) AddPodMember(podID, userID int64, role string) error {
	if _, err := s.db.Exec(`
INSERT INTO pod_members (pod_id, user_id, role)
VALUES (?, ?, ?)
`, podID, userID, role); err != nil {
		return fmt.Errorf("insert pod member: %w", err)
	}
	return nil
}

func (s *Store) GetPodMember(podID, userID int64) (*PodMember, error) {
	var member PodMember
	err := s.db.QueryRow(`
SELECT pod_id, user_id, role, joined_at
FROM pod_members
WHERE pod_id = ? AND user_id = ?
`, podID, userID).Scan(&member.PodID, &member.UserID, &member.Role, &member.JoinedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get pod member: %w", err)
	}
	return &member, nil
}

func (s *Store) IsPodMember(podID, userID int64) (bool, error) {
	var count int
	if err := s.db.QueryRow(`
SELECT COUNT(*)
FROM pod_members
WHERE pod_id = ? AND user_id = ?
`, podID, userID).Scan(&count); err != nil {
		return false, fmt.Errorf("check pod member: %w", err)
	}
	return count > 0, nil
}

func (s *Store) GetPodLeaderboard(podID int64) ([]PodLeaderboardRow, error) {
	rows, err := s.db.Query(`
SELECT
	p.name,
	pe.elo,
	COUNT(DISTINCT g.id) AS games,
	COALESCE(SUM(CASE WHEN g.id IS NOT NULL AND gp.placement = 1 THEN 1 ELSE 0 END), 0) AS wins
FROM pod_elos pe
JOIN players p ON p.id = pe.player_id
LEFT JOIN game_players gp ON gp.player_id = pe.player_id
LEFT JOIN games g ON g.id = gp.game_id AND g.pod_id = pe.pod_id
WHERE pe.pod_id = ?
GROUP BY p.id, p.name, pe.elo
ORDER BY pe.elo DESC, p.name ASC
`, podID)
	if err != nil {
		return nil, fmt.Errorf("get pod leaderboard: %w", err)
	}
	defer rows.Close()

	out := make([]PodLeaderboardRow, 0)
	for rows.Next() {
		var row PodLeaderboardRow
		if err := rows.Scan(&row.Name, &row.Elo, &row.Games, &row.Wins); err != nil {
			return nil, fmt.Errorf("scan pod leaderboard row: %w", err)
		}
		if row.Games > 0 {
			row.WinRate = (float64(row.Wins) / float64(row.Games)) * 100.0
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pod leaderboard rows: %w", err)
	}
	return out, nil
}

func (s *Store) GetOrCreatePodPlayer(podID int64, name string) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin pod player transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	playerID, err := getOrCreatePlayerID(tx, name)
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`
INSERT INTO pod_elos (pod_id, player_id, elo)
VALUES (?, ?, ?)
ON CONFLICT(pod_id, player_id) DO NOTHING
`, podID, playerID, scoring.DefaultStartingScore); err != nil {
		return 0, fmt.Errorf("insert pod elo: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit pod player transaction: %w", err)
	}
	return playerID, nil
}

func (s *Store) GetPodMemberNames(podID int64) ([]string, error) {
	rows, err := s.db.Query(`
SELECT DISTINCT candidate_name
FROM (
	SELECT p.name AS candidate_name
	FROM pod_elos pe
	JOIN players p ON p.id = pe.player_id
	WHERE pe.pod_id = ?
	UNION
	SELECT u.display_name AS candidate_name
	FROM pod_members pm
	JOIN users u ON u.id = pm.user_id
	WHERE pm.pod_id = ?
)
WHERE candidate_name != ''
ORDER BY candidate_name COLLATE NOCASE ASC
`, podID, podID)
	if err != nil {
		return nil, fmt.Errorf("get pod member names query: %w", err)
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan pod member name row: %w", err)
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pod member name rows: %w", err)
	}
	return out, nil
}

func (s *Store) RecordPodGame(podID int64, playedAt time.Time, source string, rankings []string, snapshot map[string]int) (int64, error) {
	return s.RecordPodGameWithStats(podID, playedAt, source, nil, nil, rankings, nil, snapshot)
}

func (s *Store) RecordPodGameWithStats(
	podID int64,
	playedAt time.Time,
	source string,
	turnCount *int,
	notes *string,
	rankings []string,
	playerStats []PlayerGameStat,
	snapshot map[string]int,
) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin pod game transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(`INSERT INTO games (played_at, source, pod_id) VALUES (?, ?, ?)`, playedAt.UTC(), source, podID)
	if err != nil {
		return 0, fmt.Errorf("insert pod game: %w", err)
	}
	gameID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read inserted pod game id: %w", err)
	}

	statsByPlayer := make(map[string]PlayerGameStat, len(playerStats))
	for _, stat := range playerStats {
		statsByPlayer[stat.Player] = stat
	}

	if turnCount != nil || notes != nil {
		if _, err := tx.Exec(`
INSERT INTO game_metadata (game_id, turn_count, notes)
VALUES (?, ?, ?)
`, gameID, turnCount, notes); err != nil {
			return 0, fmt.Errorf("insert game metadata: %w", err)
		}
	}

	for i, name := range rankings {
		playerID, err := getOrCreatePlayerID(tx, name)
		if err != nil {
			return 0, err
		}
		if _, err := tx.Exec(`INSERT INTO game_players (game_id, player_id, placement) VALUES (?, ?, ?)`, gameID, playerID, i+1); err != nil {
			return 0, fmt.Errorf("insert pod game player row: %w", err)
		}

		previousElo, err := getPodPlayerElo(tx, podID, playerID)
		if err != nil {
			return 0, err
		}
		nextElo, ok := snapshot[name]
		if !ok {
			nextElo = scoring.DefaultStartingScore
		}
		if _, err := tx.Exec(`
INSERT INTO pod_elos (pod_id, player_id, elo, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(pod_id, player_id) DO UPDATE SET elo = excluded.elo, updated_at = excluded.updated_at
`, podID, playerID, nextElo, time.Now().UTC()); err != nil {
			return 0, fmt.Errorf("upsert pod elo: %w", err)
		}

		userID, linked, err := getPlayerUserID(tx, playerID)
		if err != nil {
			return 0, err
		}
		if linked {
			delta := nextElo - previousElo
			if _, err := tx.Exec(`UPDATE users SET global_elo = global_elo + ? WHERE id = ?`, delta, userID); err != nil {
				return 0, fmt.Errorf("update user global elo delta: %w", err)
			}
		}

		stat, hasStats := statsByPlayer[name]
		if !hasStats {
			continue
		}
		startingLife := stat.StartingLife
		if startingLife == 0 {
			startingLife = 40
		}
		if _, err := tx.Exec(`
INSERT INTO player_game_stats (
	game_id,
	player_id,
	deck_name,
	starting_life,
	final_life,
	eliminations,
	commander_damage_dealt,
	commander_damage_taken,
	turn_eliminated
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
`, gameID, playerID, nullIfBlank(stat.DeckName), startingLife, stat.FinalLife, stat.Eliminations, stat.CommanderDamageDealt, stat.CommanderDamageTaken, stat.TurnEliminated); err != nil {
			return 0, fmt.Errorf("insert player game stats: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit pod game transaction: %w", err)
	}
	return gameID, nil
}

func (s *Store) GetGameWithStats(gameID int64) (*GameWithStats, error) {
	var (
		game         GameWithStats
		turnCountRaw sql.NullInt64
		notesRaw     sql.NullString
		podFormatRaw sql.NullString
	)
	err := s.db.QueryRow(`
SELECT
	g.id,
	g.played_at,
	g.source,
	COALESCE(g.pod_id, 0),
	COALESCE(p.format, 'commander'),
	gm.turn_count,
	gm.notes
FROM games g
LEFT JOIN pods p ON p.id = g.pod_id
LEFT JOIN game_metadata gm ON gm.game_id = g.id
WHERE g.id = ?
`, gameID).Scan(&game.ID, &game.PlayedAt, &game.Source, &game.PodID, &podFormatRaw, &turnCountRaw, &notesRaw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get game with stats query: %w", err)
	}

	game.PodFormat = "commander"
	if podFormatRaw.Valid && podFormatRaw.String != "" {
		game.PodFormat = podFormatRaw.String
	}
	if turnCountRaw.Valid {
		v := int(turnCountRaw.Int64)
		game.TurnCount = &v
	}
	if notesRaw.Valid {
		v := notesRaw.String
		game.Notes = &v
	}

	rows, err := s.db.Query(`
SELECT
	gp.placement,
	p.name,
	COALESCE(pgs.deck_name, ''),
	COALESCE(pgs.starting_life, 40),
	pgs.final_life,
	COALESCE(pgs.eliminations, 0),
	COALESCE(pgs.commander_damage_dealt, 0),
	COALESCE(pgs.commander_damage_taken, 0),
	pgs.turn_eliminated
FROM game_players gp
JOIN players p ON p.id = gp.player_id
LEFT JOIN player_game_stats pgs ON pgs.game_id = gp.game_id AND pgs.player_id = gp.player_id
WHERE gp.game_id = ?
ORDER BY gp.placement ASC
`, gameID)
	if err != nil {
		return nil, fmt.Errorf("game with stats placements query: %w", err)
	}
	defer rows.Close()

	game.Placements = make([]GamePlayerWithStats, 0)
	for rows.Next() {
		var (
			row               GamePlayerWithStats
			finalLifeRaw      sql.NullInt64
			turnEliminatedRaw sql.NullInt64
		)
		if err := rows.Scan(
			&row.Placement,
			&row.Player,
			&row.DeckName,
			&row.StartingLife,
			&finalLifeRaw,
			&row.Eliminations,
			&row.CommanderDamageDealt,
			&row.CommanderDamageTaken,
			&turnEliminatedRaw,
		); err != nil {
			return nil, fmt.Errorf("scan game with stats placement row: %w", err)
		}
		if finalLifeRaw.Valid {
			v := int(finalLifeRaw.Int64)
			row.FinalLife = &v
		}
		if turnEliminatedRaw.Valid {
			v := int(turnEliminatedRaw.Int64)
			row.TurnEliminated = &v
		}
		game.Placements = append(game.Placements, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate game with stats placement rows: %w", err)
	}

	return &game, nil
}

func (s *Store) loadGamePlacements(gameID int64) ([]GamePlacement, error) {
	rows, err := s.db.Query(`
SELECT gp.placement, p.name
FROM game_players gp
JOIN players p ON p.id = gp.player_id
WHERE gp.game_id = ?
ORDER BY gp.placement ASC
`, gameID)
	if err != nil {
		return nil, fmt.Errorf("game placements query: %w", err)
	}
	defer rows.Close()

	placements := make([]GamePlacement, 0)
	for rows.Next() {
		var placement GamePlacement
		if err := rows.Scan(&placement.Placement, &placement.Player); err != nil {
			return nil, fmt.Errorf("scan placement row: %w", err)
		}
		placements = append(placements, placement)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate placement rows: %w", err)
	}
	return placements, nil
}

func getOrCreatePlayerID(tx *sql.Tx, name string) (int64, error) {
	if _, err := tx.Exec(`INSERT INTO players (name) VALUES (?) ON CONFLICT(name) DO NOTHING`, name); err != nil {
		return 0, fmt.Errorf("insert player: %w", err)
	}

	var id int64
	if err := tx.QueryRow(`SELECT id FROM players WHERE name = ?`, name).Scan(&id); err != nil {
		return 0, fmt.Errorf("lookup player id: %w", err)
	}
	return id, nil
}

func getPodPlayerElo(tx *sql.Tx, podID, playerID int64) (int, error) {
	var elo int
	err := tx.QueryRow(`
SELECT elo
FROM pod_elos
WHERE pod_id = ? AND player_id = ?
`, podID, playerID).Scan(&elo)
	if err == sql.ErrNoRows {
		return scoring.DefaultStartingScore, nil
	}
	if err != nil {
		return 0, fmt.Errorf("lookup pod player elo: %w", err)
	}
	return elo, nil
}

func getPlayerUserID(tx *sql.Tx, playerID int64) (int64, bool, error) {
	var userID sql.NullInt64
	if err := tx.QueryRow(`SELECT user_id FROM players WHERE id = ?`, playerID).Scan(&userID); err != nil {
		return 0, false, fmt.Errorf("lookup linked user id: %w", err)
	}
	if !userID.Valid {
		return 0, false, nil
	}
	return userID.Int64, true, nil
}

func nullIfBlank(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}
