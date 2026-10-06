package web

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/wo0lien/compete/store"
)

func TestTrack(t *testing.T) {
	racers := []store.Racer{{Username: "lou", Pawn: "#a", Wins: 8}, {Username: "mika", Pawn: "#b", Wins: 4}, {Username: "sam", Pawn: "#c", Wins: 0}}
	cells := track(racers)
	if len(cells) != 9 || cells[0].Label != "GO" || cells[8].Label != "★" {
		t.Fatalf("cells = %+v", cells)
	}
	where := map[string]int{}
	for i, c := range cells {
		for _, p := range c.Pawns {
			where[p] = i
		}
	}
	if want := map[string]int{"#a": 8, "#b": 4, "#c": 0}; !reflect.DeepEqual(where, want) {
		t.Fatalf("positions = %v, want %v", where, want)
	}
	if got := track([]store.Racer{{Pawn: "#a"}}); len(got[0].Pawns) != 1 {
		t.Fatal("with no wins everyone stays on GO")
	}
}

func TestLeaderboardHistoryBoardPages(t *testing.T) {
	ts, alice, bob := twoPlayers(t)
	submit(t, ts, alice, tusmoAlice)
	submit(t, ts, bob, tusmoBob)
	submit(t, ts, alice, strings.Replace(tusmoAlice, "#70", "#71", 1)) // closes #70

	_, body := get(t, bob, ts.URL+"/g/1/leaderboard")
	if !strings.Contains(body, "class=\"track\"") || !strings.Contains(body, "Tusmo") || !strings.Contains(body, "music") {
		t.Fatalf("leaderboard page:\n%s", body)
	}
	_, body = get(t, bob, ts.URL+"/g/1/leaderboard?tag=music")
	if strings.Contains(body, "<td>alice</td>") {
		t.Fatal("music filter still lists Tusmo stats")
	}
	_, body = get(t, bob, ts.URL+"/g/1/history?game=tusmo&variant=")
	if !strings.Contains(body, "/g/1/board?game=tusmo&amp;variant=&amp;puzzle=71") || !strings.Contains(body, "puzzle=70") {
		t.Fatalf("history page:\n%s", body)
	}
	_, body = get(t, bob, ts.URL+"/g/1/board?game=tusmo&variant=&puzzle=71")
	if !strings.Contains(body, "hidden-tile") {
		t.Fatal("bob has not played #71: it must be hidden")
	}
	c := newClient(t)
	signup(t, ts, c, "carol")
	for _, p := range []string{"/g/1/leaderboard", "/g/1/history?game=tusmo", "/g/1/board?game=tusmo&puzzle=70"} { // Review Focus 2
		if resp, _ := get(t, c, ts.URL+p); resp.StatusCode != http.StatusNotFound {
			t.Errorf("outsider %s = %d, want 404", p, resp.StatusCode)
		}
	}
}

func TestTagFilterMarksSelection(t *testing.T) {
	ts, _, bob := twoPlayers(t)
	_, body := get(t, bob, ts.URL+"/g/1/leaderboard?tag=music")
	if !strings.Contains(body, `class="filters"`) || !strings.Contains(body, `aria-current="page">music</a>`) {
		t.Fatalf("tag filter should be a .filters nav with music selected:\n%s", body)
	}
}
