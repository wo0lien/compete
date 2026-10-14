package store

import (
	"errors"
	"slices"
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
