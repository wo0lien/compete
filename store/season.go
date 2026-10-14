package store

import (
	"errors"
	"slices"
	"strings"
)

// ErrSeasonDays: a season lasts short (3), normal (7) or long (14) days.
var ErrSeasonDays = errors.New("season length must be 3, 7 or 14 days")

var seasonLengths = []int{3, 7, 14}

// seasonN is the finish line: what a strong player (beating 3/4 of the group
// on every puzzle) earns in days, at least 1. Integer form of
// ceil(days × games × (1 + 3/4 × (members − 1))).
func seasonN(days, games, members int) int {
	return max(1, (days*games*(3*members+1)+3)/4)
}

// SeasonDays is the length the group's next season is sized for.
func (s *Store) SeasonDays(groupID int64) (int, error) {
	var d int
	err := s.db.QueryRow("SELECT season_days FROM groups WHERE id = ?", groupID).Scan(&d)
	return d, err
}

// SetSeasonDays changes the length of the next seasons; the running one keeps its N.
func (s *Store) SetSeasonDays(groupID int64, days int) error {
	if !slices.Contains(seasonLengths, days) {
		return ErrSeasonDays
	}
	_, err := s.db.Exec("UPDATE groups SET season_days = ? WHERE id = ?", days, groupID)
	return err
}

// scored is one counted puzzle: its day and each member's points on it.
type scored struct {
	day string
	pts map[int64]int
}

// points scores one puzzle: 1 for playing plus 1 per member strictly beaten.
// Members who did not play are beaten by everyone who did. ranks maps each
// member who played to their RANK() (1 = best, ties share).
func points(ranks map[int64]int, members int) map[int64]int {
	out := make(map[int64]int, len(ranks))
	for u, rk := range ranks {
		p := 1 + members - len(ranks)
		for _, other := range ranks {
			if other > rk {
				p++
			}
		}
		out[u] = p
	}
	return out
}

// walk adds counted puzzles up in day order. The first day at whose end
// someone's total reaches n is the final day; later days belong to the next
// season and are left out.
func walk(ps []scored, n int) (totals map[int64]int, final string) {
	ps = slices.Clone(ps)
	slices.SortStableFunc(ps, func(a, b scored) int { return strings.Compare(a.day, b.day) })
	totals = map[int64]int{}
	for i, p := range ps {
		for u, v := range p.pts {
			totals[u] += v
		}
		if i+1 < len(ps) && ps[i+1].day == p.day {
			continue // the day isn't over yet
		}
		for _, v := range totals {
			if v >= n {
				return totals, p.day
			}
		}
	}
	return totals, ""
}

// winners are the members with the top total, by id; nobody on an empty race.
func winners(totals map[int64]int) []int64 {
	top := 0
	for _, v := range totals {
		top = max(top, v)
	}
	var out []int64
	for u, v := range totals {
		if v == top && top > 0 {
			out = append(out, u)
		}
	}
	slices.Sort(out)
	return out
}
