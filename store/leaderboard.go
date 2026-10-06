package store

import (
	"database/sql"
	"encoding/json"
)

type Stat struct {
	Username string
	Plays    int
	Wins     int // first in the group on a puzzle (ties included), fails excluded
	Fails    int
	AvgScore sql.NullFloat64 // over non-failed plays
}

func (st Stat) WinRate() int { return st.Wins * 100 / st.Plays }

type GameStats struct {
	Game, Variant string
	Stats         []Stat
}

// closedRanked ranks every result of group ?1 on closed puzzles only (older than
// the newest puzzle per game/variant). ?2 is NULL for all games, or a JSON array
// of game ids to keep.
const closedRanked = `
	WITH gr AS (
		SELECT r.user_id, r.game, r.variant, r.puzzle_id, r.score, r.tiebreak
		FROM results r JOIN memberships m ON m.user_id = r.user_id AND m.group_id = ?1
		WHERE ?2 IS NULL OR r.game IN (SELECT value FROM json_each(?2))
	),
	cur AS (SELECT game, variant, MAX(puzzle_id) AS open_id FROM gr GROUP BY game, variant),
	ranked AS (
		SELECT gr.*, RANK() OVER (
			PARTITION BY gr.game, gr.variant, gr.puzzle_id
			ORDER BY gr.score IS NULL, gr.score, gr.tiebreak IS NULL, gr.tiebreak) AS rk
		FROM gr JOIN cur ON cur.game = gr.game AND cur.variant = gr.variant
		WHERE gr.puzzle_id < cur.open_id
	)`

// Leaderboard aggregates closed puzzles only: those older than the newest puzzle
// seen per game/variant in the group. The open puzzle is excluded so totals never
// reveal scores the Board reveal rule still hides.
func (s *Store) Leaderboard(groupID int64) ([]GameStats, error) {
	rows, err := s.db.Query(closedRanked+`
		SELECT ranked.game, ranked.variant, u.username, COUNT(*) AS plays,
		       SUM(rk = 1 AND score IS NOT NULL) AS wins, SUM(score IS NULL) AS fails,
		       AVG(score) AS avg_score
		FROM ranked JOIN users u ON u.id = ranked.user_id
		GROUP BY ranked.game, ranked.variant, ranked.user_id
		ORDER BY ranked.game, ranked.variant, wins DESC, avg_score IS NULL, avg_score,
		         u.username COLLATE NOCASE`, groupID, nil)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameStats
	for rows.Next() {
		var game, variant string
		var st Stat
		if err := rows.Scan(&game, &variant, &st.Username, &st.Plays, &st.Wins, &st.Fails, &st.AvgScore); err != nil {
			return nil, err
		}
		if n := len(out); n == 0 || out[n-1].Game != game || out[n-1].Variant != variant {
			out = append(out, GameStats{Game: game, Variant: variant})
		}
		out[len(out)-1].Stats = append(out[len(out)-1].Stats, st)
	}
	return out, rows.Err()
}

type Racer struct {
	Username, Pawn string
	Wins           int
}

// Race counts, for every member, wins on closed puzzles of the given games
// (nil = all games). It feeds the race track.
func (s *Store) Race(groupID int64, gameIDs []string) ([]Racer, error) {
	var filter any // NULL = every game
	if gameIDs != nil {
		b, err := json.Marshal(gameIDs)
		if err != nil {
			return nil, err
		}
		filter = string(b)
	}
	rows, err := s.db.Query(closedRanked+`
		SELECT u.username, u.pawn, COALESCE(SUM(ranked.rk = 1 AND ranked.score IS NOT NULL), 0) AS wins
		FROM memberships m JOIN users u ON u.id = m.user_id
		LEFT JOIN ranked ON ranked.user_id = m.user_id
		WHERE m.group_id = ?1
		GROUP BY u.id
		ORDER BY wins DESC, u.username COLLATE NOCASE`, groupID, filter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Racer
	for rows.Next() {
		var r Racer
		if err := rows.Scan(&r.Username, &r.Pawn, &r.Wins); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
