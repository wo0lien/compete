package games

import (
	"reflect"
	"testing"
)

func TestTravleFixtures(t *testing.T) {
	checkFixture(t, "travle/win.txt", []Result{{
		Game: "travle", PuzzleID: 1392, Score: new(0), Detail: map[string]any{"grid": "✅🟩🟩✅✅"},
	}})
	checkFixture(t, "travle/fail.txt", []Result{{
		Game: "travle", PuzzleID: 1392, Tiebreak: new(3), Detail: map[string]any{"grid": "🟥🟥🟥🟥🟥🟥🟥🟧✅✅"},
	}})
}

// Assumed variant format (#travle_<code>); replace with a real fixture once one is shared.
func TestTravleVariant(t *testing.T) {
	got := Parse("#travle_usa #12 +2\n✅🟧✅\nhttps://travle.earth/usa")
	want := []Result{{Game: "travle", Variant: "usa", PuzzleID: 12, Score: new(2), Detail: map[string]any{"grid": "✅🟧✅"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
