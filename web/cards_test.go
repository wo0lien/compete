package web

import (
	"regexp"
	"strings"
	"testing"

	"github.com/wo0lien/compete/store"
)

// A game whose variants share one puzzle (Songless categories) gets one card per
// puzzle, opened on the first variant the viewer hasn't played; other games keep
// one card per board.
func TestCardsGroupVariants(t *testing.T) {
	boards := []store.Board{
		{Game: "songless", Variant: "All", PuzzleID: 410, Revealed: true},
		{Game: "travle", Variant: "", PuzzleID: 1398},
		{Game: "songless", Variant: "Rock", PuzzleID: 410},
		{Game: "travle", Variant: "usa", PuzzleID: 179},
		{Game: "songless", Variant: "Hip Hop", PuzzleID: 410, Revealed: true},
		{Game: "songless", Variant: "All", PuzzleID: 409, Revealed: true},
	}
	cs := cards(boards)
	if len(cs) != 4 {
		t.Fatalf("cards = %d, want 4 (songless #410, travle, travle usa, songless #409): %+v", len(cs), cs)
	}
	s := cs[0]
	if s.Game != "songless" || s.PuzzleID != 410 || len(s.Boards) != 3 || s.Selected != 1 || !s.ToPlay() {
		t.Fatalf("songless #410 card = %+v, want 3 variants, Rock selected, to play", s)
	}
	if cs[1].Game != "travle" || len(cs[1].Boards) != 1 || cs[2].Game != "travle" || len(cs[2].Boards) != 1 {
		t.Fatalf("travle variants must stay separate cards: %+v %+v", cs[1], cs[2])
	}
	if c := cs[3]; len(c.Boards) != 1 || c.Selected != 0 || c.ToPlay() {
		t.Fatalf("songless #409 card = %+v, want one revealed board", c)
	}
}

// All variants played: the card opens on the first one.
func TestCardsAllPlayedSelectsFirst(t *testing.T) {
	cs := cards([]store.Board{
		{Game: "songless", Variant: "All", PuzzleID: 410, Revealed: true},
		{Game: "songless", Variant: "Rock", PuzzleID: 410, Revealed: true},
	})
	if len(cs) != 1 || cs[0].Selected != 0 {
		t.Fatalf("cards = %+v, want one card on its first variant", cs)
	}
}

// The group page draws Songless #404 as one card: chips (shown by JS), one panel
// per category, opened on the category the viewer still has to play; without JS
// every panel shows under its own category name.
func TestGroupPageVariantCard(t *testing.T) {
	ts, alice, bob := twoPlayers(t)
	submit(t, ts, alice, "Songless #404\n\n🟨🟨🟩⬛⬛ [All]\n⬜🟩⬛⬛⬛ [Rock]")
	submit(t, ts, bob, "Songless #404\n\n🟨🟩⬛⬛⬛ [All]\n⬜⬜🟩⬛⬛ [Rock]\n🟩⬛⬛⬛⬛ [Hip Hop]")
	_, body := get(t, alice, ts.URL+"/g/1")
	if n := strings.Count(body, "data-variants"); n != 1 {
		t.Fatalf("variant cards = %d, want 1:\n%s", n, body)
	}
	if n := strings.Count(body, `class="variant-panel"`); n != 3 {
		t.Fatalf("variant panels = %d, want 3:\n%s", n, body)
	}
	for _, want := range []string{
		`<div class="variant-tabs" role="tablist" hidden>`,
		`aria-selected="true" data-variant="`, // one chip selected…
		`<h3 class="variant-name">Hip Hop</h3>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("group page misses %q", want)
		}
	}
	if !regexp.MustCompile(`aria-selected="true" data-variant="\d+">Hip Hop`).MatchString(body) {
		t.Errorf("Hip Hop (not played by alice) is not the selected chip:\n%s", body)
	}
	if strings.Count(body, "Songless #404") != 1 {
		t.Errorf("card title repeated per category:\n%s", body)
	}
}
