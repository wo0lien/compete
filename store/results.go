package store

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/wo0lien/compete/games"
)

var ErrDuplicate = errors.New("already submitted")

// ErrNoResults is AddResults called with nothing to store.
var ErrNoResults = errors.New("no results")

// ErrFuturePuzzle is a result for a puzzle not out yet (above today+1).
var ErrFuturePuzzle = errors.New("puzzle not out yet")

// AddResults stores parsed results for a user and returns the ones that were new.
// Already-stored results are skipped, so re-sharing a Songless text after playing
// one more category only adds that category. ErrDuplicate if nothing was new.
func (s *Store) AddResults(userID int64, raw string, rs []games.Result) ([]games.Result, error) {
	if len(rs) == 0 {
		return nil, ErrNoResults
	}
	now := s.Now()
	for _, r := range rs {
		g, _ := games.ByID(r.Game)
		if today, ok := g.Today(r.Variant, now); ok && r.PuzzleID > today+1 {
			return nil, ErrFuturePuzzle
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var added []games.Result
	for _, r := range rs {
		var detail any // NULL when there is no detail
		if r.Detail != nil {
			b, err := json.Marshal(r.Detail)
			if err != nil {
				return nil, err
			}
			detail = string(b)
		}
		res, err := tx.Exec(`
			INSERT INTO results(user_id, game, variant, puzzle_id, score, tiebreak, detail, raw)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
			userID, r.Game, r.Variant, r.PuzzleID, r.Score, r.Tiebreak, detail, raw)
		if err != nil {
			return nil, err
		}
		if n, _ := res.RowsAffected(); n == 1 {
			added = append(added, r)
		}
	}
	if len(added) == 0 {
		return nil, ErrDuplicate
	}
	return added, tx.Commit()
}

type Entry struct {
	Username string
	Played   bool
	Hidden   bool // played, but the viewer has not submitted this puzzle yet
	Score    *int
	Tiebreak *int
	Grid     string
	Pawn     string
	Me       bool // the viewer's own row
	Rank     int  // 1-based, shared on ties like the leaderboard's RANK(); 0 if not played
}

type Board struct {
	Game, Variant string
	PuzzleID      int
	Revealed      bool // the viewer has submitted this puzzle
	Entries       []Entry
}

// Board ranks every group member on one puzzle. Until the viewer has submitted
// that same (game, variant, puzzle), other members' results are hidden and
// listed alphabetically, because rank order alone would leak scores.
func (s *Store) Board(groupID, viewerID int64, game, variant string, puzzleID int) (Board, error) {
	b := Board{Game: game, Variant: variant, PuzzleID: puzzleID}
	rows, err := s.db.Query(`
		SELECT u.id, u.username, u.pawn, r.id IS NOT NULL, r.score, r.tiebreak,
		       COALESCE(json_extract(r.detail, '$.grid'), '')
		FROM memberships m
		JOIN users u ON u.id = m.user_id
		LEFT JOIN results r ON r.user_id = m.user_id
		     AND r.game = ? AND r.variant = ? AND r.puzzle_id = ?
		WHERE m.group_id = ?
		ORDER BY r.id IS NULL, r.score IS NULL, r.score, r.tiebreak IS NULL, r.tiebreak,
		         u.username COLLATE NOCASE`,
		game, variant, puzzleID, groupID)
	if err != nil {
		return Board{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var e Entry
		var uid int64
		if err := rows.Scan(&uid, &e.Username, &e.Pawn, &e.Played, &e.Score, &e.Tiebreak, &e.Grid); err != nil {
			return Board{}, err
		}
		e.Me = uid == viewerID
		if e.Me && e.Played {
			b.Revealed = true
		}
		if e.Played { // rows come in rank order
			e.Rank = len(b.Entries) + 1
			if n := len(b.Entries); n > 0 && b.Entries[n-1].Played &&
				eqInt(b.Entries[n-1].Score, e.Score) && eqInt(b.Entries[n-1].Tiebreak, e.Tiebreak) {
				e.Rank = b.Entries[n-1].Rank
			}
		}
		b.Entries = append(b.Entries, e)
	}
	if err := rows.Err(); err != nil {
		return Board{}, err
	}
	if !b.Revealed {
		for i := range b.Entries {
			if e := &b.Entries[i]; e.Played {
				e.Hidden, e.Score, e.Tiebreak, e.Grid, e.Rank = true, nil, nil, "", 0
			}
		}
		sort.SliceStable(b.Entries, func(i, j int) bool {
			a, c := b.Entries[i], b.Entries[j]
			if a.Played != c.Played {
				return a.Played
			}
			return strings.ToLower(a.Username) < strings.ToLower(c.Username)
		})
	}
	return b, nil
}

// eqInt compares nullable ints; two NULLs are equal, as in SQL's RANK().
func eqInt(a, b *int) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

// CurrentBoards returns, for each game/variant played in the group, the board of
// today's puzzle, from the game's schedule: it shows before anyone has played it.
// Without a schedule, "current" is the newest puzzle_id among members' results.
// A newer id than today's (today+1, a player already past midnight) wins; one
// further ahead is ignored.
func (s *Store) CurrentBoards(groupID, viewerID int64) ([]Board, error) {
	rows, err := s.db.Query(`
		SELECT r.game, r.variant, MAX(r.puzzle_id) FROM results r
		JOIN memberships m ON m.user_id = r.user_id AND m.group_id = ?
		GROUP BY r.game, r.variant ORDER BY r.game, r.variant`, groupID)
	if err != nil {
		return nil, err
	}
	var keys []Board
	for rows.Next() {
		var k Board
		if err := rows.Scan(&k.Game, &k.Variant, &k.PuzzleID); err != nil {
			rows.Close()
			return nil, err
		}
		keys = append(keys, k)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// ponytail: one query per board; batch if groups ever play dozens of games.
	boards := make([]Board, 0, len(keys))
	now := s.Now()
	for _, k := range keys {
		g, _ := games.ByID(k.Game)
		// Ids beyond today+1 can only be rows stored before AddResults refused them (#4).
		if today, ok := g.Today(k.Variant, now); ok && (today > k.PuzzleID || k.PuzzleID > today+1) {
			k.PuzzleID = today
		}
		b, err := s.Board(groupID, viewerID, k.Game, k.Variant, k.PuzzleID)
		if err != nil {
			return nil, err
		}
		boards = append(boards, b)
	}
	return boards, nil
}

// Puzzles lists the puzzle ids of a game/variant played in the group, newest first.
func (s *Store) Puzzles(groupID int64, game, variant string) ([]int, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT r.puzzle_id FROM results r
		JOIN memberships m ON m.user_id = r.user_id AND m.group_id = ?
		WHERE r.game = ? AND r.variant = ?
		ORDER BY r.puzzle_id DESC`, groupID, game, variant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ToPlay lists the games (ids, in games.All order) the user submitted in the
// last 14 days and has not yet played today for at least one of those variants.
// Games without a known schedule are left out.
func (s *Store) ToPlay(userID int64) ([]string, error) {
	now := s.Now()
	rows, err := s.db.Query(`SELECT DISTINCT game, variant FROM results WHERE user_id = ? AND submitted_at > ?`,
		userID, now.Add(-14*24*time.Hour).Unix())
	if err != nil {
		return nil, err
	}
	type key struct{ game, variant string }
	var keys []key
	for rows.Next() {
		var k key
		if err := rows.Scan(&k.game, &k.variant); err != nil {
			rows.Close()
			return nil, err
		}
		keys = append(keys, k)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	todo := map[string]bool{}
	for _, k := range keys {
		g, _ := games.ByID(k.game)
		today, ok := g.Today(k.variant, now)
		if !ok || todo[k.game] {
			continue
		}
		var played bool // a today+1 share counts too
		err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM results WHERE user_id = ? AND game = ? AND variant = ? AND puzzle_id >= ?)`,
			userID, k.game, k.variant, today).Scan(&played)
		if err != nil {
			return nil, err
		}
		todo[k.game] = !played
	}
	var out []string
	for _, g := range games.All {
		if todo[g.ID] {
			out = append(out, g.ID)
		}
	}
	return out, nil
}
