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
	// Variant tags are travle_<map id>, numbered from each map's own start (USA #1209
	// on 2026-10-09; the format is from the site's share code).
	got := Parse("#travle_usa #1209 +2\n✅🟧✅\nhttps://travle.earth/usa")
	want := []Result{{Game: "travle", Variant: "usa", PuzzleID: 1209, Score: new(2), Detail: map[string]any{"grid": "✅🟧✅"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// US-state maps (usaca, usatx…) are variants too, with their own numbering.
func TestTravleStateVariant(t *testing.T) {
	rs := Parse("#travle_usaca #179 (2 away)\n🟥🟧✅")
	if len(rs) != 1 || rs[0].Variant != "usaca" || rs[0].PuzzleID != 179 || rs[0].Score != nil {
		t.Fatalf("Parse = %+v, want a failed usaca #179", rs)
	}
	if today, ok := travle.Today("usaca", at("2026-10-09T12:00:00Z")); !ok || today != rs[0].PuzzleID {
		t.Fatalf("Today(usaca) = %d, %v; want the shared #179", today, ok)
	}
}
