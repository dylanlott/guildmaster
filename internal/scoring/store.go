package scoring

import (
	"errors"
	"maps"
	"math"
	"sync"
)

// Simple in-memory scoring store for player Elo ratings.
type Store struct {
	mu     sync.RWMutex
	scores map[string]int
}

// NewStore creates a new in-memory store.
func NewStore() *Store {
	return &Store{scores: make(map[string]int)}
}

// GetAll returns a copy of all scores.
func (s *Store) GetAll() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]int, len(s.scores))
	maps.Copy(out, s.scores)
	return out
}

// Set sets a player's score.
func (s *Store) Set(player string, score int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scores[player] = score
}

// ApplyDeltas applies integer deltas to players (adds delta to existing or default 1500).
func (s *Store) ApplyDeltas(deltas map[string]int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	const defaultStartingScore = 1500
	for player, delta := range deltas {
		cur, ok := s.scores[player]
		if !ok {
			cur = defaultStartingScore
		}
		s.scores[player] = cur + delta
	}
}

// ScoreGame computes Elo deltas for a finished game (players ordered by finish: winner first).
// It returns the computed deltas but does not persist them; caller can persist via ApplyDeltas.
func ScoreGame(game []string, K int, D float64, snapshot map[string]int) (map[string]int, error) {
	if len(game) < 2 {
		return nil, errors.New("need at least two players")
	}

	numPlayers := len(game)
	ratings := make([]float64, numPlayers)
	for i := 0; i < numPlayers; i++ {
		if v, ok := snapshot[game[i]]; ok {
			ratings[i] = float64(v)
		} else {
			ratings[i] = 1500.0
		}
	}

	deltasF := make([]float64, numPlayers)
	for i := 0; i < numPlayers; i++ {
		for j := i + 1; j < numPlayers; j++ {
			RA := ratings[i]
			RB := ratings[j]
			EA := 1.0 / (1.0 + math.Pow(10, (RB-RA)/D))
			deltaA := float64(K) * (1.0 - EA)
			deltasF[i] += deltaA
			deltasF[j] -= deltaA
		}
	}

	deltas := make(map[string]int, numPlayers)
	for i := 0; i < numPlayers; i++ {
		deltas[game[i]] = int(math.Round(deltasF[i]))
	}
	return deltas, nil
}
