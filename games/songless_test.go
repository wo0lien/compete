package games

import (
	"reflect"
	"testing"
)

func TestSonglessFixture(t *testing.T) {
	checkFixture(t, "songless/404.txt", []Result{
		{Game: "songless", Variant: "All", PuzzleID: 404, Score: new(3), Detail: map[string]any{"grid": "🟨🟨🟩⬛⬛"}},
		{Game: "songless", Variant: "Rock", PuzzleID: 404, Score: new(2), Detail: map[string]any{"grid": "⬜🟩⬛⬛⬛"}},
		{Game: "songless", Variant: "Hip Hop", PuzzleID: 404, Detail: map[string]any{"grid": "⬜⬜⬜⬜⬜"}},
	})
}

// Review Focus 1: wrong guesses (🟥), U+FE0F after ⬛ and CRLF.
func TestSonglessMessyPaste(t *testing.T) {
	got := Parse("Songless #405\r\n\r\n🟥🟨🟩⬛️⬛️ [Pop]\r\n")
	want := []Result{{Game: "songless", Variant: "Pop", PuzzleID: 405, Score: new(3), Detail: map[string]any{"grid": "🟥🟨🟩⬛⬛"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestSonglessHeaderWithoutRowsIsUnsupported(t *testing.T) {
	if got := Parse("Songless #405\n\nPlay Today's Game: https://less.gg/songless"); got != nil {
		t.Fatalf("got %+v, want nil", got)
	}
}

// Two shares pasted together: only the first share's rows belong to its puzzle.
func TestSonglessTwoSharesPastedTogether(t *testing.T) {
	got := Parse("Songless #404\n\n🟨🟨🟩⬛⬛ [All]\n\nSongless #405\n\n🟩⬛⬛⬛⬛ [Pop]")
	want := []Result{{Game: "songless", Variant: "All", PuzzleID: 404, Score: new(3), Detail: map[string]any{"grid": "🟨🟨🟩⬛⬛"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
