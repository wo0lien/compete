package games

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	fermiHead  = regexp.MustCompile(`^Fermi · No\. (\d+)`)
	fermiScore = regexp.MustCompile(`(?m)^(\d+(?:\.\d+)?)× score`)
	fermiRow   = regexp.MustCompile(`(?m)^\d\d +\d+(?:\.\d+)?×$`)
)

var fermi = Game{ID: "fermi", Name: "Fermi", URL: "https://fermi.gg", Lang: "en",
	Tags: []string{"numbers", "english"}, Fail: "fail", Parse: parseFermi, show: showFermi,
	today: daily("America/New_York", "2026-07-27")}

// parseFermi: the score is how far off the guesses were, as a multiple of the
// answers (geometric mean of the three, ≥ 1×), stored in hundredths. Fermi
// shows 3 significant digits, so hundredths keep every digit.
func parseFermi(text string) []Result {
	h := fermiHead.FindStringSubmatch(text)
	s := fermiScore.FindStringSubmatch(text)
	if h == nil || s == nil {
		return nil
	}
	f, _ := strconv.ParseFloat(s[1], 64) // regexp-validated
	grid := strings.Join(fermiRow.FindAllString(text, -1), "\n")
	return []Result{{Game: "fermi", PuzzleID: atoi(h[1]), Score: new(int(math.Round(f * 100))), Detail: map[string]any{"grid": grid}}}
}

// showFermi prints hundredths with 3 significant digits, as the game does.
func showFermi(h int) string {
	switch {
	case h < 1000:
		return fmt.Sprintf("%d.%02d×", h/100, h%100)
	case h < 10000:
		return fmt.Sprintf("%d.%d×", h/100, h%100/10)
	}
	return fmt.Sprintf("%d×", h/100)
}
