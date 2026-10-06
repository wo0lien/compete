package games

import (
	"reflect"
	"strings"
	"testing"
)

func TestTusmoFixtures(t *testing.T) {
	checkFixture(t, "tusmo/win.txt", []Result{{
		Game: "tusmo", PuzzleID: 70, Score: new(3), Tiebreak: new(35),
		Detail: map[string]any{
			"grid":    "🟥🟥🟥⬛🟨🟨🟨⬛\n🟥🟥🟥⬛⬛🟨🟨🟨\n🟥🟥🟥🟥🟥🟥🟥🟥",
			"top_pct": 6,
		},
	}})
	checkFixture(t, "tusmo/fail.txt", []Result{{
		Game: "tusmo", PuzzleID: 70, Tiebreak: new(21),
		Detail: map[string]any{"grid": strings.TrimSuffix(strings.Repeat("🟥🟥🟥⬛⬛🟨🟨🟨\n", 6), "\n")},
	}})
}

// Review Focus 1: lowercase header, CRLF and U+FE0F as pasted from iOS.
func TestTusmoMessyPaste(t *testing.T) {
	got := Parse("tusmo #71 2/6 - 1:05\r\n\r\n🟥⬛️\r\n🟥🟥\r\n")
	want := []Result{{
		Game: "tusmo", PuzzleID: 71, Score: new(2), Tiebreak: new(65),
		Detail: map[string]any{"grid": "🟥⬛\n🟥🟥"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
