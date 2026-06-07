package scoring

import (
	"reflect"
	"testing"
)

func TestScoreGameSimple(t *testing.T) {
	snapshot := map[string]int{"A": 1500, "B": 1500}
	deltas, err := ScoreGame([]string{"A", "B"}, 40, 800, snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deltas) != 2 {
		t.Fatalf("expected 2 deltas, got %d", len(deltas))
	}
	if deltas["A"] <= 0 || deltas["B"] >= 0 {
		t.Fatalf("unexpected signs for deltas: %v", deltas)
	}
	if deltas["A"]+deltas["B"] != 0 {
		t.Fatalf("deltas should sum to zero, got %v", deltas)
	}

	s := NewStore()
	s.ApplyDeltas(deltas)
	scores := s.GetAll()
	expected := map[string]int{"A": 1500 + deltas["A"], "B": 1500 + deltas["B"]}
	if !reflect.DeepEqual(scores, expected) {
		t.Fatalf("expected scores %v, got %v", expected, scores)
	}
}

func TestScoreGameRequiresAtLeastTwoPlayers(t *testing.T) {
	_, err := ScoreGame([]string{"A"}, DefaultK, DefaultD, map[string]int{"A": 1500})
	if err == nil {
		t.Fatal("expected error for single-player game")
	}
}

func TestScoreGameUnknownPlayersStartAtDefault(t *testing.T) {
	snapshot := map[string]int{}
	deltas, err := ScoreGame([]string{"NewA", "NewB"}, DefaultK, DefaultD, snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deltas["NewA"]+deltas["NewB"] != 0 {
		t.Fatalf("expected deltas to balance, got %v", deltas)
	}

	ApplyDeltas(snapshot, deltas)
	if snapshot["NewA"] != DefaultStartingScore+deltas["NewA"] {
		t.Fatalf("unexpected score for NewA: %d", snapshot["NewA"])
	}
	if snapshot["NewB"] != DefaultStartingScore+deltas["NewB"] {
		t.Fatalf("unexpected score for NewB: %d", snapshot["NewB"])
	}
}

func TestScoreGameThreePlayerOrderingAndConservation(t *testing.T) {
	snapshot := map[string]int{"A": 1700, "B": 1500, "C": 1300}
	deltas, err := ScoreGame([]string{"A", "B", "C"}, DefaultK, DefaultD, snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	total := 0
	for _, delta := range deltas {
		total += delta
	}
	if total != 0 {
		t.Fatalf("expected zero-sum deltas, got %d (%v)", total, deltas)
	}

	if deltas["A"] <= deltas["B"] {
		t.Fatalf("winner should gain most in this setup: %v", deltas)
	}
	if deltas["C"] >= 0 {
		t.Fatalf("last place should not gain rating: %v", deltas)
	}
}

func TestStoreReplaceAllAndGetAllIsolation(t *testing.T) {
	s := NewStore()
	initial := map[string]int{"A": 1600, "B": 1400}
	s.ReplaceAll(initial)

	initial["A"] = 9999
	out := s.GetAll()
	if out["A"] != 1600 {
		t.Fatalf("store should copy input map, got %d", out["A"])
	}

	out["B"] = 1111
	second := s.GetAll()
	if second["B"] != 1400 {
		t.Fatalf("GetAll should return a copy, got %d", second["B"])
	}
}

func TestApplyDeltasUsesDefaultForMissingPlayers(t *testing.T) {
	snapshot := map[string]int{"A": 1500}
	ApplyDeltas(snapshot, map[string]int{"A": 10, "B": -5})

	expected := map[string]int{"A": 1510, "B": 1495}
	if !reflect.DeepEqual(snapshot, expected) {
		t.Fatalf("expected %v, got %v", expected, snapshot)
	}
}
