package games

import (
	"regexp"
	"strings"
	"time"
)

var travleRe = regexp.MustCompile(`^#travle(?:_(\w+))? #(\d+) (?:\+(\d+)|\((\d+) away\))`)

var travle = Game{ID: "travle", Name: "Travle", URL: "https://travle.earth", Lang: "en",
	Tags: []string{"geography", "english"}, ScoreFmt: "+%d", Fail: "fail", TiebreakKind: "away", Parse: parseTravle,
	today: travleToday}

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

// Travle resets at each player's own midnight; Paris stands in for everyone.
// ponytail: one timezone for all players; per-member timezones if groups spread out.
var travleLoc = mustLoad("Europe/Paris")

// travleStarts: each map has its own numbering (from the site's bundle, 2026-10-09).
var travleStarts = map[string]string{
	"":    "2022-12-15",
	"fra": "2023-06-19", "gbr": "2023-06-19", "irl": "2023-06-19", "ita": "2023-06-19",
	"rus": "2023-06-19", "ukr": "2023-06-19", "usa": "2023-06-19",
	"jpn": "2023-06-21",
	"mda": "2023-08-24", "prt": "2023-08-24", "rou": "2023-08-24",
	"ind": "2023-10-05", "mex": "2023-10-05", "benelux": "2023-10-05", "bra": "2023-10-05", "esp": "2023-10-05",
	"che": "2023-11-01", "bgr": "2023-11-01",
	"chn": "2024-01-09", "col": "2024-01-09", "deu": "2024-01-09", "nord": "2024-01-09", "tur": "2024-01-09",
	"arg": "2024-04-11", "aut": "2024-04-11",
	"kor":  "2024-05-04",
	"lotr": "2026-03-26",
	"ldn":  "2026-04-06",
	"grc":  "2026-04-07", "coast": "2026-04-07",
	"par":  "2026-04-09",
	"1880": "2026-05-13",
}

func travleToday(variant string, now time.Time) (int, bool) {
	start, ok := travleStarts[variant]
	if !ok && len(variant) == 5 && strings.HasPrefix(variant, "usa") { // US states: usaca, usatx…
		start, ok = "2026-04-14", true
	}
	if !ok {
		return 0, false // the weekly challenge, or a map we don't know
	}
	return dayNumber(now, travleLoc, start), true
}
