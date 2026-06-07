package server

import (
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/dylanlott/guildmaster/internal/db"
	"github.com/dylanlott/guildmaster/internal/scoring"
)

type playerTimelinePoint struct {
	PlayedAt time.Time
	Elo      int
}

type playerRecentGameRow struct {
	GameID      int64
	PlayedAt    time.Time
	Placement   int
	Opponents   string
	EloChange   int
	EloAfter    int
	PlayerCount int
}

type headToHeadRow struct {
	Opponent string
	Wins     int
	Losses   int
}

type gameReplayState struct {
	Games        []db.GameRecord
	DeltasByGame map[int64]map[string]int
}

func placementsToRankings(placements []db.GamePlacement) []string {
	rankings := make([]string, 0, len(placements))
	for _, placement := range placements {
		rankings = append(rankings, placement.Player)
	}
	return rankings
}

func (s *Server) replayState() (*gameReplayState, error) {
	if s.dbStore == nil {
		return &gameReplayState{Games: []db.GameRecord{}, DeltasByGame: map[int64]map[string]int{}}, nil
	}

	games, err := s.dbStore.ListAllGamesChronological()
	if err != nil {
		return nil, err
	}
	deltasByGame := make(map[int64]map[string]int, len(games))
	snapshot := make(map[string]int)

	for _, game := range games {
		rankings := placementsToRankings(game.Placements)
		if len(rankings) < 2 {
			continue
		}
		deltas, err := scoring.ScoreGame(rankings, s.K, s.D, snapshot)
		if err != nil {
			return nil, err
		}
		deltasByGame[game.ID] = deltas
		scoring.ApplyDeltas(snapshot, deltas)
	}

	return &gameReplayState{Games: games, DeltasByGame: deltasByGame}, nil
}

func (s *Server) HandlePlayerPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.dbStore == nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}

	rawName := strings.TrimPrefix(r.URL.Path, "/players/")
	if strings.TrimSpace(rawName) == "" {
		http.NotFound(w, r)
		return
	}
	name, err := url.PathUnescape(rawName)
	if err != nil {
		http.Error(w, "invalid player name", http.StatusBadRequest)
		return
	}

	profile, err := s.dbStore.GetPlayerProfile(name)
	if err != nil {
		http.Error(w, "failed to load profile: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if profile == nil {
		http.NotFound(w, r)
		return
	}

	playerStats, err := s.dbStore.GetPlayerStats()
	if err != nil {
		http.Error(w, "failed to load player stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	replay, err := s.replayState()
	if err != nil {
		http.Error(w, "failed to replay history: "+err.Error(), http.StatusInternalServerError)
		return
	}

	timeline := make([]playerTimelinePoint, 0)
	recentGames := make([]playerRecentGameRow, 0)
	headToHead := map[string]*headToHeadRow{}

	snapshot := make(map[string]int)
	for _, game := range replay.Games {
		deltas := replay.DeltasByGame[game.ID]
		if deltas == nil {
			continue
		}
		rankings := placementsToRankings(game.Placements)
		for _, player := range rankings {
			if _, ok := snapshot[player]; !ok {
				snapshot[player] = scoring.DefaultStartingScore
			}
		}

		targetPlacement := 0
		for _, placement := range game.Placements {
			if placement.Player == name {
				targetPlacement = placement.Placement
				break
			}
		}

		if targetPlacement > 0 {
			change := deltas[name]
			after := snapshot[name] + change
			timeline = append(timeline, playerTimelinePoint{PlayedAt: game.PlayedAt, Elo: after})

			opponents := make([]string, 0, len(game.Placements)-1)
			for _, placement := range game.Placements {
				if placement.Player == name {
					continue
				}
				opponents = append(opponents, placement.Player)
				row, ok := headToHead[placement.Player]
				if !ok {
					row = &headToHeadRow{Opponent: placement.Player}
					headToHead[placement.Player] = row
				}
				if targetPlacement < placement.Placement {
					row.Wins++
				} else {
					row.Losses++
				}
			}

			recentGames = append(recentGames, playerRecentGameRow{
				GameID:      game.ID,
				PlayedAt:    game.PlayedAt,
				Placement:   targetPlacement,
				Opponents:   strings.Join(opponents, ", "),
				EloChange:   change,
				EloAfter:    after,
				PlayerCount: len(game.Placements),
			})
		}

		scoring.ApplyDeltas(snapshot, deltas)
	}

	sort.Slice(recentGames, func(i, j int) bool {
		if recentGames[i].PlayedAt.Equal(recentGames[j].PlayedAt) {
			return recentGames[i].GameID > recentGames[j].GameID
		}
		return recentGames[i].PlayedAt.After(recentGames[j].PlayedAt)
	})
	if len(recentGames) > 15 {
		recentGames = recentGames[:15]
	}

	headRows := make([]headToHeadRow, 0, len(headToHead))
	for _, row := range headToHead {
		headRows = append(headRows, *row)
	}
	sort.Slice(headRows, func(i, j int) bool {
		left := headRows[i].Wins - headRows[i].Losses
		right := headRows[j].Wins - headRows[j].Losses
		if left == right {
			return headRows[i].Opponent < headRows[j].Opponent
		}
		return left > right
	})

	t, err := template.New("player.tmpl").Funcs(template.FuncMap{
		"humanDate": humanDate,
		"deltaClass": func(v int) string {
			switch {
			case v > 0:
				return "pos"
			case v < 0:
				return "neg"
			default:
				return "zero"
			}
		},
	}).ParseFS(tmplFS, "player.tmpl")
	if err != nil {
		http.Error(w, "template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	stats := playerStats[name]
	losses := stats.TotalGames - stats.Wins
	if losses < 0 {
		losses = 0
	}

	data := struct {
		Name        string
		Elo         int
		Wins        int
		Losses      int
		WinRate     float64
		AvgPlace    float64
		Timeline    []playerTimelinePoint
		RecentGames []playerRecentGameRow
		HeadToHead  []headToHeadRow
		Nav         navData
	}{
		Name:        profile.Name,
		Elo:         profile.Elo,
		Wins:        stats.Wins,
		Losses:      losses,
		WinRate:     stats.WinRate,
		AvgPlace:    stats.AvgPlacement,
		Timeline:    timeline,
		RecentGames: recentGames,
		HeadToHead:  headRows,
		Nav:         navForRequest(r),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "template execute error: "+err.Error(), http.StatusInternalServerError)
	}
}
