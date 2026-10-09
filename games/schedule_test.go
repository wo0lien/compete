package games

import (
	"testing"
	"time"
)

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

// Verified on 2026-10-09 against each game's live site (see the spec).
func TestToday(t *testing.T) {
	for _, tc := range []struct {
		game, variant, now string
		want               int
		ok                 bool
	}{
		{"tusmo", "", "2026-10-09T12:00:00Z", 73, true},
		{"tusmo", "", "2026-10-06T12:00:00Z", 70, true},
		{"tusmo", "", "2026-10-08T23:59:59Z", 72, true}, // resets at 00:00 UTC
		{"tusmo", "", "2026-10-09T00:00:00Z", 73, true},
		{"songless", "Rock", "2026-10-09T12:00:00Z", 407, true},
		{"songless", "All", "2026-10-06T12:00:00Z", 404, true},
		{"songless", "", "2026-10-09T03:59:59Z", 406, true}, // 23:59 in New York (EDT)
		{"songless", "", "2026-10-09T04:00:00Z", 407, true},
		{"songless", "", "2026-11-15T04:59:59Z", 443, true}, // EST after 2026-11-01: reset at 05:00 UTC
		{"songless", "", "2026-11-15T05:00:00Z", 444, true},
		{"travle", "", "2026-10-09T12:00:00Z", 1395, true},
		{"travle", "", "2026-10-06T12:00:00Z", 1392, true},
		{"travle", "", "2026-10-08T21:59:59Z", 1394, true}, // 23:59 in Paris (CEST)
		{"travle", "", "2026-10-08T22:00:00Z", 1395, true},
		{"travle", "usa", "2026-10-09T12:00:00Z", 1209, true},
		{"travle", "usaca", "2026-10-09T12:00:00Z", 179, true},
		{"travle", "challenge", "2026-10-09T12:00:00Z", 0, false},
		{"travle", "atlantis", "2026-10-09T12:00:00Z", 0, false},
	} {
		g, _ := ByID(tc.game)
		got, ok := g.Today(tc.variant, at(tc.now))
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s %q at %s = %d, %v; want %d, %v", tc.game, tc.variant, tc.now, got, ok, tc.want, tc.ok)
		}
	}
}
