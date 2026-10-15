package web

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/wo0lien/compete/store"
)

// Pawns stand at points × 8 / N, below ★ until they reach N.
func TestTrack(t *testing.T) {
	se := store.Season{N: 40, Racers: []store.Racer{
		{Pawn: "#a", Points: 40}, {Pawn: "#b", Points: 39}, {Pawn: "#c", Points: 20}, {Pawn: "#d", Points: 0}, {Pawn: "#e", Points: 55},
	}}
	cells := track(se)
	if len(cells) != 9 || cells[0].Label != "GO" || cells[8].Label != "★" {
		t.Fatalf("cells = %+v", cells)
	}
	where := map[string]int{}
	for i, c := range cells {
		for _, p := range c.Pawns {
			where[p] = i
		}
	}
	if want := map[string]int{"#a": 8, "#b": 7, "#c": 4, "#d": 0, "#e": 8}; !reflect.DeepEqual(where, want) {
		t.Fatalf("positions = %v, want %v", where, want)
	}
	// Cells show the points needed to stand on them, so the legend matches the track.
	var labels []string
	for _, c := range track(store.Season{N: 53})[1:8] {
		labels = append(labels, c.Label)
	}
	if want := []string{"7", "14", "20", "27", "34", "40", "47"}; !reflect.DeepEqual(labels, want) {
		t.Fatalf("labels = %v, want %v", labels, want)
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
	if !strings.Contains(body, "Season 1") {
		t.Fatalf("leaderboard lacks the season header:\n%s", body)
	}
	_, body = get(t, bob, ts.URL+"/g/1/leaderboard?tag=music")
	if strings.Contains(body, "<td>alice</td>") {
		t.Fatal("music filter still lists Tusmo stats")
	}
	_, body = get(t, bob, ts.URL+"/g/1/history?game=tusmo&variant=")
	if !strings.Contains(body, "/g/1/board?game=tusmo&amp;variant=&amp;puzzle=71") || !strings.Contains(body, "puzzle=70") {
		t.Fatalf("history page:\n%s", body)
	}
	// #71 (tomorrow's, alice only): dated, winner hidden from bob; #70 (today's): both played
	if !strings.Contains(body, "#71</a> <span class=\"muted\">7 Oct</span>") || !strings.Contains(body, "Play it to see who won") ||
		!strings.Contains(body, "#70</a> <span class=\"muted\">Today</span>") || !strings.Contains(body, `class="winner"`) {
		t.Fatalf("history rows lack date or winner:\n%s", body)
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

// Stats tables scroll inside their panel instead of widening the page on a phone.
func TestLeaderboardTableWrapped(t *testing.T) {
	ts, alice, bob := twoPlayers(t)
	submit(t, ts, alice, tusmoAlice)
	submit(t, ts, bob, tusmoBob)
	submit(t, ts, alice, strings.Replace(tusmoAlice, "#70", "#71", 1)) // closes #70, so stats exist
	_, body := get(t, bob, ts.URL+"/g/1/leaderboard")
	if !strings.Contains(body, `<div class="table-wrap"><table>`) {
		t.Fatalf("stats table not wrapped:\n%s", body)
	}
}
