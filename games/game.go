// Package games turns share texts from daily games into normalized results.
package games

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Result is one normalized outcome parsed from a share text.
type Result struct {
	Game     string
	Variant  string // "" for games with one result per puzzle
	PuzzleID int
	Score    *int           // lower is better; nil = failed
	Tiebreak *int           // lower is better; nil = none
	Detail   map[string]any // stored as JSON; "grid" holds this result's own grid lines
}

// Game describes a supported game and how to parse and display its results.
type Game struct {
	ID, Name, URL string
	Lang          string                                          // the game's language, BCP 47 ("fr", "en")
	Tags          []string                                        // topics for filtering: "words", "music", "geography", "french"…
	ScoreFmt      string                                          // formats a score, e.g. "%d/6"
	Fail          string                                          // shown for a failed result, e.g. "X/6"
	TiebreakKind  string                                          // "time" (seconds), "away" (countries short) or ""
	Parse         func(text string) []Result                      // nil = not this game
	today         func(variant string, now time.Time) (int, bool) // nil = no known schedule
}

// Today is the number of today's puzzle for variant at now, from the game's
// published numbering and reset time. false: no known schedule.
func (g Game) Today(variant string, now time.Time) (int, bool) {
	if g.today == nil {
		return 0, false
	}
	return g.today(variant, now)
}

// ShowScore renders a stored score for display.
func (g Game) ShowScore(score *int) string {
	if score == nil {
		return g.Fail
	}
	return fmt.Sprintf(g.ScoreFmt, *score)
}

// ShowTiebreak renders a stored tiebreak, or "" when the game has none.
func (g Game) ShowTiebreak(t *int) string {
	switch {
	case t == nil:
		return ""
	case g.TiebreakKind == "time":
		return fmt.Sprintf("%d:%02d", *t/60, *t%60)
	case g.TiebreakKind == "away":
		return fmt.Sprintf("%d away", *t)
	}
	return ""
}

// AllTags returns every tag used by a supported game, sorted.
func AllTags() []string {
	seen := map[string]bool{}
	var tags []string
	for _, g := range All {
		for _, t := range g.Tags {
			if !seen[t] {
				seen[t] = true
				tags = append(tags, t)
			}
		}
	}
	sort.Strings(tags)
	return tags
}

// WithTag returns the IDs of the games carrying tag.
func WithTag(tag string) []string {
	var ids []string
	for _, g := range All {
		for _, t := range g.Tags {
			if t == tag {
				ids = append(ids, g.ID)
				break
			}
		}
	}
	return ids
}

// All lists the supported games, in matching order.
var All = []Game{tusmo, songless, travle}

// normalizer cleans share text pasted from any platform: some keyboards append
// U+FE0F to ⬛/⬜, Windows/iOS paste CRLF, copies can carry a BOM, zero-width
// spaces or NBSPs.
var normalizer = strings.NewReplacer("\uFE0F", "", "\uFEFF", "", "\u200B", "", "\u00A0", " ", "\r\n", "\n")

// Normalize cleans share text before parsing.
func Normalize(text string) string {
	return strings.TrimSpace(normalizer.Replace(text))
}

// Parse returns the results of the first game that recognizes text, or nil.
// Headers are anchored at the start, so lines the member typed above the share
// text are skipped one at a time.
func Parse(text string) []Result {
	text = Normalize(text)
	for {
		for _, g := range All {
			if rs := g.Parse(text); rs != nil {
				return rs
			}
		}
		i := strings.IndexByte(text, '\n')
		if i < 0 {
			return nil
		}
		text = strings.TrimSpace(text[i+1:])
	}
}

// ByID returns the game with the given id.
func ByID(id string) (Game, bool) {
	for _, g := range All {
		if g.ID == id {
			return g, true
		}
	}
	return Game{}, false
}

// atoi is only called on regexp-validated digit groups.
func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// gridOf returns the lines made only of non-ASCII symbols: the emoji grid.
func gridOf(text string) string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(l)
		if l != "" && strings.IndexFunc(l, func(r rune) bool { return r < 128 }) == -1 {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}
