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
	// Winner should gain positive, loser negative and sums to zero
	if deltas["A"] <= 0 || deltas["B"] >= 0 {
		t.Fatalf("unexpected signs for deltas: %v", deltas)
	}
	if deltas["A"]+deltas["B"] != 0 {
		t.Fatalf("deltas should sum to zero, got %v", deltas)
	}

	// Roundtrip apply deltas
	s := NewStore()
	s.ApplyDeltas(deltas)
	scores := s.GetAll()
	expected := map[string]int{"A": 1500 + deltas["A"], "B": 1500 + deltas["B"]}
	if !reflect.DeepEqual(scores, expected) {
		t.Fatalf("expected scores %v, got %v", expected, scores)
	}
}
