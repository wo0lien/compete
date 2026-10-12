package web

import (
	"reflect"
	"strings"
	"testing"
)

// squares maps each grid emoji to a palette class, line by line.
func TestSquares(t *testing.T) {
	got := squares("🟥🟨⬛\n✅🟧⬜🟩")
	want := [][]string{{"sq red", "sq yellow", "sq miss"}, {"sq green", "sq orange", "sq empty", "sq green"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("squares = %v, want %v", got, want)
	}
}

// The emoji presentation selector some keyboards add is not a square.
func TestSquaresIgnoresVariationSelector(t *testing.T) {
	if got := squares("⬛️🟩"); !reflect.DeepEqual(got, [][]string{{"sq miss", "sq green"}}) {
		t.Fatalf("squares = %v", got)
	}
}

// Anything outside the map (a game adds 🟦, or text): nil, so the page keeps the emoji.
func TestSquaresUnknownFallsBack(t *testing.T) {
	for _, g := range []string{"🟥🟦", "🎵🟩", "", "🟥\nX"} {
		if got := squares(g); got != nil {
			t.Errorf("squares(%q) = %v, want nil", g, got)
		}
	}
}

// A revealed board draws each grid as squares behind "Show grid".
func TestBoardGridSquares(t *testing.T) {
	ts, alice, bob := twoPlayers(t)
	submit(t, ts, alice, tusmoAlice)
	submit(t, ts, bob, tusmoBob)
	_, body := get(t, alice, ts.URL+"/g/1")
	if !strings.Contains(body, `<details class="squares"><summary>Show grid</summary><div class="sq-row"><span class="sq `) || strings.Contains(body, `<pre class="grid">`) {
		t.Fatalf("grid not drawn as squares:\n%s", body)
	}
}
