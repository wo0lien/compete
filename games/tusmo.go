package games

import "regexp"

var tusmoRe = regexp.MustCompile(`(?i)^TUSMO #(\d+) ([1-6X])/6 - (\d+):(\d\d)(?: - top (\d+)%)?`)

var tusmo = Game{ID: "tusmo", Name: "Tusmo", URL: "https://www.tusmo.xyz", Lang: "fr",
	Tags: []string{"words", "french"}, ScoreFmt: "%d/6", Fail: "X/6", TiebreakKind: "time", Parse: parseTusmo}

// parseTusmo reads the header line: attempts (X = failed), time as tiebreak,
// optional percentile. The grid is kept for display only.
func parseTusmo(text string) []Result {
	m := tusmoRe.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	r := Result{
		Game:     "tusmo",
		PuzzleID: atoi(m[1]),
		Tiebreak: new(atoi(m[3])*60 + atoi(m[4])),
		Detail:   map[string]any{"grid": gridOf(text)},
	}
	if m[2] != "X" && m[2] != "x" {
		r.Score = new(atoi(m[2]))
	}
	if m[5] != "" {
		r.Detail["top_pct"] = atoi(m[5])
	}
	return []Result{r}
}
