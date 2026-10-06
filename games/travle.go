package games

import "regexp"

var travleRe = regexp.MustCompile(`^#travle(?:_(\w+))? #(\d+) (?:\+(\d+)|\((\d+) away\))`)

var travle = Game{ID: "travle", Name: "Travle", URL: "https://travle.earth", Lang: "en",
	Tags: []string{"geography", "english"}, ScoreFmt: "+%d", Fail: "fail", TiebreakKind: "away", Parse: parseTravle}

// parseTravle: "+N" is a win with N guesses over the optimal path; "(N away)"
// is a fail N countries short, which ranks fails by closeness.
func parseTravle(text string) []Result {
	m := travleRe.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	r := Result{Game: "travle", Variant: m[1], PuzzleID: atoi(m[2]), Detail: map[string]any{"grid": gridOf(text)}}
	if m[3] != "" {
		r.Score = new(atoi(m[3]))
	} else {
		r.Tiebreak = new(atoi(m[4]))
	}
	return []Result{r}
}
