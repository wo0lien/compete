package games

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	songlessHead = regexp.MustCompile(`^Songless #(\d+)`)
	songlessRow  = regexp.MustCompile(`(?m)^([⬜🟥🟨🟩⬛]{5}) \[([^\]\n]+)\][ \t]*$`)
)

var songless = Game{ID: "songless", Name: "Songless", URL: "https://lessgames.com/songless", Lang: "en",
	Tags: []string{"music", "english"}, ScoreFmt: "%d/5", Fail: "X/5", Parse: parseSongless,
	today: daily("America/New_York", "2025-08-29")}

// Each category row becomes a board in the group, so a paste is bounded: real
// Songless categories are short genre names and few per day.
const (
	songlessMaxRows = 12
	songlessMaxName = 30 // runes
)

// parseSongless returns one Result per category row. Score is the position of
// 🟩; a row without 🟩 is a fail.
func parseSongless(text string) []Result {
	h := songlessHead.FindStringSubmatch(text)
	if h == nil {
		return nil
	}
	// Rows belong to this header only: stop at a second pasted share.
	body := text[len(h[0]):]
	if i := strings.Index(body, "Songless #"); i >= 0 {
		body = body[:i]
	}
	var rs []Result
	for _, m := range songlessRow.FindAllStringSubmatch(body, -1) {
		if utf8.RuneCountInString(m[2]) > songlessMaxName {
			continue
		}
		if len(rs) == songlessMaxRows {
			break
		}
		r := Result{Game: "songless", Variant: m[2], PuzzleID: atoi(h[1]), Detail: map[string]any{"grid": m[1]}}
		for i, sq := range []rune(m[1]) {
			if sq == '🟩' {
				r.Score = new(i + 1)
				break
			}
		}
		rs = append(rs, r)
	}
	return rs
}
