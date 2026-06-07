package db

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dylanlott/guildmaster/internal/scoring"
)

func TestNewStoreAppliesMigrationsAndWAL(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "guildmaster-test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	var journalMode string
	if err := store.db.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("query journal mode: %v", err)
	}
	if strings.ToLower(journalMode) != "wal" {
		t.Fatalf("expected journal_mode=wal, got %q", journalMode)
	}

	var applied int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatalf("count schema migrations: %v", err)
	}
	if applied != len(schemaMigrations) {
		t.Fatalf("expected %d migrations to be applied, got %d", len(schemaMigrations), applied)
	}

	expectedIndexes := []string{
		"idx_players_elo_name",
		"idx_games_played_at_id",
		"idx_game_players_game_id_placement",
		"idx_game_players_player_id_game_id",
	}
	for _, indexName := range expectedIndexes {
		var found int
		if err := store.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, indexName).Scan(&found); err != nil {
			t.Fatalf("query index %s: %v", indexName, err)
		}
		if found != 1 {
			t.Fatalf("expected index %s to exist", indexName)
		}
	}
}

func TestGetPlayerStats(t *testing.T) {
	store, err := NewStore("file:guildmaster_stats_test?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	snapshot := map[string]int{}
	record := func(rankings []string, at time.Time) {
		deltas, err := scoring.ScoreGame(rankings, scoring.DefaultK, scoring.DefaultD, snapshot)
		if err != nil {
			t.Fatalf("ScoreGame() failed: %v", err)
		}
		scoring.ApplyDeltas(snapshot, deltas)
		if _, err := store.RecordGame(at, "test", rankings, snapshot); err != nil {
			t.Fatalf("RecordGame() failed: %v", err)
		}
	}

	record([]string{"Alice", "Bob", "Cara"}, time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC))
	record([]string{"Bob", "Alice", "Cara"}, time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC))

	stats, err := store.GetPlayerStats()
	if err != nil {
		t.Fatalf("GetPlayerStats() failed: %v", err)
	}

	assertStats(t, stats["Alice"], 2, 1, 50.0, 1.5)
	assertStats(t, stats["Bob"], 2, 1, 50.0, 1.5)
	assertStats(t, stats["Cara"], 2, 0, 0.0, 3.0)
}

func TestListUsersAndUpdateUserRole(t *testing.T) {
	store, err := NewStore("file:guildmaster_users_test?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	owner, err := store.CreateUser("owner", "Owner", "hash")
	if err != nil {
		t.Fatalf("CreateUser(owner) failed: %v", err)
	}
	admin, err := store.CreateUser("admin", "Admin", "hash")
	if err != nil {
		t.Fatalf("CreateUser(admin) failed: %v", err)
	}

	if err := store.UpdateUserRole(admin.ID, "admin"); err != nil {
		t.Fatalf("UpdateUserRole(admin) failed: %v", err)
	}

	users, err := store.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers() failed: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if users[0].ID != owner.ID || users[0].Role != "owner" {
		t.Fatalf("unexpected first user: %#v", users[0])
	}
	if users[1].ID != admin.ID || users[1].Role != "admin" {
		t.Fatalf("unexpected second user: %#v", users[1])
	}
}

func TestUpdateUserRolePreventsRemovingLastOwner(t *testing.T) {
	store, err := NewStore("file:guildmaster_last_owner_test?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	owner, err := store.CreateUser("owner", "Owner", "hash")
	if err != nil {
		t.Fatalf("CreateUser(owner) failed: %v", err)
	}

	if err := store.UpdateUserRole(owner.ID, "admin"); err == nil {
		t.Fatalf("expected last owner demotion to fail")
	}

	reloaded, err := store.GetUserByID(owner.ID)
	if err != nil {
		t.Fatalf("GetUserByID(owner) failed: %v", err)
	}
	if reloaded.Role != "owner" {
		t.Fatalf("expected last owner to remain owner, got %q", reloaded.Role)
	}
}

func TestPodStoreMethodsAndRecordPodGame(t *testing.T) {
	store, err := NewStore("file:guildmaster_pods_test?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	owner, err := store.CreateUser("owner", "Owner", "hash")
	if err != nil {
		t.Fatalf("CreateUser(owner) failed: %v", err)
	}
	aliceUser, err := store.CreateUser("alice", "Alice", "hash")
	if err != nil {
		t.Fatalf("CreateUser(alice) failed: %v", err)
	}
	bobUser, err := store.CreateUser("bob", "Bob", "hash")
	if err != nil {
		t.Fatalf("CreateUser(bob) failed: %v", err)
	}

	pod, err := store.CreatePod(owner.ID, "friday-pod", "Friday Pod", "weekly games", "commander", true)
	if err != nil {
		t.Fatalf("CreatePod() failed: %v", err)
	}
	if pod == nil || pod.Slug != "friday-pod" || !pod.Public || pod.Format != "commander" {
		t.Fatalf("unexpected pod: %#v", pod)
	}

	if err := store.AddPodMember(pod.ID, owner.ID, "owner"); err != nil {
		t.Fatalf("AddPodMember(owner) failed: %v", err)
	}
	if err := store.AddPodMember(pod.ID, aliceUser.ID, "member"); err != nil {
		t.Fatalf("AddPodMember(alice) failed: %v", err)
	}
	if err := store.AddPodMember(pod.ID, bobUser.ID, "member"); err != nil {
		t.Fatalf("AddPodMember(bob) failed: %v", err)
	}

	member, err := store.GetPodMember(pod.ID, owner.ID)
	if err != nil {
		t.Fatalf("GetPodMember() failed: %v", err)
	}
	if member == nil || member.Role != "owner" {
		t.Fatalf("expected owner role, got %#v", member)
	}

	isMember, err := store.IsPodMember(pod.ID, aliceUser.ID)
	if err != nil {
		t.Fatalf("IsPodMember() failed: %v", err)
	}
	if !isMember {
		t.Fatalf("expected alice to be a pod member")
	}

	userPods, err := store.ListUserPods(owner.ID)
	if err != nil {
		t.Fatalf("ListUserPods() failed: %v", err)
	}
	if len(userPods) != 1 || userPods[0].ID != pod.ID {
		t.Fatalf("unexpected user pods: %#v", userPods)
	}

	publicPods, err := store.ListPublicPods()
	if err != nil {
		t.Fatalf("ListPublicPods() failed: %v", err)
	}
	if len(publicPods) != 1 || publicPods[0].ID != pod.ID {
		t.Fatalf("unexpected public pods: %#v", publicPods)
	}

	bySlug, err := store.GetPodBySlug("friday-pod")
	if err != nil {
		t.Fatalf("GetPodBySlug() failed: %v", err)
	}
	if bySlug == nil || bySlug.ID != pod.ID {
		t.Fatalf("unexpected pod by slug: %#v", bySlug)
	}

	alicePlayerID, err := store.GetOrCreatePodPlayer(pod.ID, "Alice")
	if err != nil {
		t.Fatalf("GetOrCreatePodPlayer(Alice) failed: %v", err)
	}
	bobPlayerID, err := store.GetOrCreatePodPlayer(pod.ID, "Bob")
	if err != nil {
		t.Fatalf("GetOrCreatePodPlayer(Bob) failed: %v", err)
	}
	if _, err := store.GetOrCreatePodPlayer(pod.ID, "Guest"); err != nil {
		t.Fatalf("GetOrCreatePodPlayer(Guest) failed: %v", err)
	}

	if _, err := store.db.Exec(`UPDATE players SET user_id = ? WHERE id = ?`, aliceUser.ID, alicePlayerID); err != nil {
		t.Fatalf("link Alice player to user: %v", err)
	}
	if _, err := store.db.Exec(`UPDATE players SET user_id = ? WHERE id = ?`, bobUser.ID, bobPlayerID); err != nil {
		t.Fatalf("link Bob player to user: %v", err)
	}
	if err := store.UpdateUserGlobalElo(aliceUser.ID, 1620); err != nil {
		t.Fatalf("seed Alice global elo failed: %v", err)
	}
	if err := store.UpdateUserGlobalElo(bobUser.ID, 1410); err != nil {
		t.Fatalf("seed Bob global elo failed: %v", err)
	}

	rankings := []string{"Alice", "Guest", "Bob"}
	snapshot := map[string]int{
		"Alice": 1500,
		"Guest": 1500,
		"Bob":   1500,
	}
	deltas, err := scoring.ScoreGame(rankings, scoring.DefaultK, scoring.DefaultD, snapshot)
	if err != nil {
		t.Fatalf("ScoreGame() failed: %v", err)
	}
	scoring.ApplyDeltas(snapshot, deltas)

	if _, err := store.RecordPodGame(pod.ID, time.Date(2026, 3, 8, 9, 0, 0, 0, time.UTC), "test", rankings, snapshot); err != nil {
		t.Fatalf("RecordPodGame() failed: %v", err)
	}

	leaderboard, err := store.GetPodLeaderboard(pod.ID)
	if err != nil {
		t.Fatalf("GetPodLeaderboard() failed: %v", err)
	}
	if len(leaderboard) != 3 {
		t.Fatalf("expected 3 leaderboard rows, got %d", len(leaderboard))
	}

	found := map[string]PodLeaderboardRow{}
	for _, row := range leaderboard {
		found[row.Name] = row
	}
	if found["Alice"].Games != 1 || found["Alice"].Wins != 1 {
		t.Fatalf("unexpected Alice leaderboard row: %#v", found["Alice"])
	}
	if found["Guest"].Games != 1 || found["Bob"].Games != 1 {
		t.Fatalf("unexpected Guest/Bob rows: guest=%#v bob=%#v", found["Guest"], found["Bob"])
	}

	updatedAlice, err := store.GetUserByID(aliceUser.ID)
	if err != nil {
		t.Fatalf("GetUserByID(alice) failed: %v", err)
	}
	updatedBob, err := store.GetUserByID(bobUser.ID)
	if err != nil {
		t.Fatalf("GetUserByID(bob) failed: %v", err)
	}
	if updatedAlice.GlobalElo != 1620+deltas["Alice"] {
		t.Fatalf("alice global elo mismatch: got %d want %d", updatedAlice.GlobalElo, 1620+deltas["Alice"])
	}
	if updatedBob.GlobalElo != 1410+deltas["Bob"] {
		t.Fatalf("bob global elo mismatch: got %d want %d", updatedBob.GlobalElo, 1410+deltas["Bob"])
	}

	memberNames, err := store.GetPodMemberNames(pod.ID)
	if err != nil {
		t.Fatalf("GetPodMemberNames() failed: %v", err)
	}
	if len(memberNames) < 3 {
		t.Fatalf("expected member names to include members and players, got %#v", memberNames)
	}

	turnCount := 11
	notes := "long grindy game"
	finalLife := 34
	eliminatedTurn := 8
	statsGameRankings := []string{"Alice", "Bob", "Guest Two"}
	statsSnapshot := map[string]int{"Alice": found["Alice"].Elo, "Bob": found["Bob"].Elo, "Guest Two": 1500}
	statsDeltas, err := scoring.ScoreGame(statsGameRankings, scoring.DefaultK, scoring.DefaultD, statsSnapshot)
	if err != nil {
		t.Fatalf("ScoreGame(stats) failed: %v", err)
	}
	scoring.ApplyDeltas(statsSnapshot, statsDeltas)

	statsGameID, err := store.RecordPodGameWithStats(
		pod.ID,
		time.Date(2026, 3, 8, 11, 0, 0, 0, time.UTC),
		"test-stats",
		&turnCount,
		&notes,
		statsGameRankings,
		[]PlayerGameStat{
			{
				Player:               "Alice",
				DeckName:             "Atraxa",
				StartingLife:         40,
				FinalLife:            &finalLife,
				Eliminations:         2,
				CommanderDamageDealt: 19,
				CommanderDamageTaken: 4,
			},
			{
				Player:         "Guest Two",
				StartingLife:   40,
				TurnEliminated: &eliminatedTurn,
			},
		},
		statsSnapshot,
	)
	if err != nil {
		t.Fatalf("RecordPodGameWithStats() failed: %v", err)
	}

	gameWithStats, err := store.GetGameWithStats(statsGameID)
	if err != nil {
		t.Fatalf("GetGameWithStats() failed: %v", err)
	}
	if gameWithStats == nil {
		t.Fatalf("expected game with stats")
	}
	if gameWithStats.PodFormat != "commander" {
		t.Fatalf("unexpected pod format: %q", gameWithStats.PodFormat)
	}
	if gameWithStats.TurnCount == nil || *gameWithStats.TurnCount != turnCount {
		t.Fatalf("turn count mismatch: %#v", gameWithStats.TurnCount)
	}
	if gameWithStats.Notes == nil || *gameWithStats.Notes != notes {
		t.Fatalf("notes mismatch: %#v", gameWithStats.Notes)
	}
	if len(gameWithStats.Placements) != 3 {
		t.Fatalf("expected 3 placements with stats, got %d", len(gameWithStats.Placements))
	}
	if gameWithStats.Placements[0].DeckName != "Atraxa" {
		t.Fatalf("expected first placement deck name, got %#v", gameWithStats.Placements[0])
	}
}

func assertStats(t *testing.T, got PlayerStats, totalGames, wins int, winRate, avgPlacement float64) {
	t.Helper()
	if got.TotalGames != totalGames {
		t.Fatalf("total games mismatch: got %d, want %d", got.TotalGames, totalGames)
	}
	if got.Wins != wins {
		t.Fatalf("wins mismatch: got %d, want %d", got.Wins, wins)
	}
	if math.Abs(got.WinRate-winRate) > 0.0001 {
		t.Fatalf("win rate mismatch: got %.4f, want %.4f", got.WinRate, winRate)
	}
	if math.Abs(got.AvgPlacement-avgPlacement) > 0.0001 {
		t.Fatalf("avg placement mismatch: got %.4f, want %.4f", got.AvgPlacement, avgPlacement)
	}
}
