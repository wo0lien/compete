package games

import "testing"

// Shared 2026-10-14: the daily score is the geometric mean of the three
// factors, stored in hundredths (26.8× → 2680).
func TestFermiFixture(t *testing.T) {
	checkFixture(t, "fermi/daily.txt", []Result{{
		Game: "fermi", PuzzleID: 80, Score: new(2680), Detail: map[string]any{"grid": "01  2.75×\n02  11.2×\n03  625×"},
	}})
}

// Shared 2026-10-15: a guess far off prints with a thousands separator.
func TestFermiThousands(t *testing.T) {
	checkFixture(t, "fermi/thousands.txt", []Result{{
		Game: "fermi", PuzzleID: 81, Score: new(1300), Detail: map[string]any{"grid": "01  354,294×\n02  1.30×\n03  1.69×"},
	}})
}

func TestFermiNeedsScore(t *testing.T) {
	if got := Parse("Fermi · No. 80\n01  2.75×"); got != nil {
		t.Fatalf("Parse without score line = %+v, want nil", got)
	}
}
