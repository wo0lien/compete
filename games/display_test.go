package games

import (
	"reflect"
	"testing"
)

func TestShowScoreAndTiebreak(t *testing.T) {
	cases := []struct {
		game          Game
		score, tb     *int
		wantS, wantTB string
	}{
		{tusmo, new(3), new(65), "3/6", "1:05"},
		{tusmo, nil, new(21), "X/6", "0:21"},
		{songless, new(2), nil, "2/5", ""},
		{songless, nil, nil, "X/5", ""},
		{travle, new(0), nil, "+0", ""},
		{travle, nil, new(3), "fail", "3 away"},
	}
	for _, c := range cases {
		if got := c.game.ShowScore(c.score); got != c.wantS {
			t.Errorf("%s ShowScore = %q, want %q", c.game.ID, got, c.wantS)
		}
		if got := c.game.ShowTiebreak(c.tb); got != c.wantTB {
			t.Errorf("%s ShowTiebreak = %q, want %q", c.game.ID, got, c.wantTB)
		}
	}
}

func TestTags(t *testing.T) {
	for _, g := range All {
		if len(g.Tags) == 0 {
			t.Errorf("%s has no tags", g.ID)
		}
	}
	if got, want := AllTags(), []string{"english", "french", "geography", "music", "words"}; !reflect.DeepEqual(got, want) {
		t.Errorf("AllTags = %v, want %v", got, want)
	}
	if got, want := WithTag("english"), []string{"songless", "travle"}; !reflect.DeepEqual(got, want) {
		t.Errorf("WithTag(english) = %v, want %v", got, want)
	}
	if got := WithTag("nope"); len(got) != 0 {
		t.Errorf("WithTag(nope) = %v, want empty", got)
	}
}
