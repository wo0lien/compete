package web

import (
	"github.com/wo0lien/compete/games"
	"github.com/wo0lien/compete/store"
)

// card is one panel of the group page: one board, or every variant of one
// puzzle for a game whose variants share a card.
type card struct {
	Group    int64 // for the history links
	Game     string
	PuzzleID int
	Boards   []store.Board
	Selected int // the variant shown first: the first one the viewer hasn't played
}

// ToPlay: the viewer still has a variant of this card to play.
func (c card) ToPlay() bool { return !c.Boards[c.Selected].Revealed }

// cards groups boards into cards, keeping the boards' order (a card sits
// where its first board was).
func cards(boards []store.Board) []card {
	var out []card
	at := map[[2]any]int{} // game, puzzle → index in out, for grouping games
	for _, b := range boards {
		g, _ := games.ByID(b.Game)
		k := [2]any{b.Game, b.PuzzleID}
		if i, ok := at[k]; ok && g.VariantsInOneCard {
			out[i].Boards = append(out[i].Boards, b)
			continue
		}
		if g.VariantsInOneCard {
			at[k] = len(out)
		}
		out = append(out, card{Game: b.Game, PuzzleID: b.PuzzleID, Boards: []store.Board{b}})
	}
	for i := range out {
		for j, b := range out[i].Boards {
			if !b.Revealed {
				out[i].Selected = j
				break
			}
		}
	}
	return out
}
