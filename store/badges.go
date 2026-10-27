package store

import (
	"database/sql"
	"errors"
	"slices"
	"time"
)

// KindLonely: the only member of the group to play that day.
const KindLonely = "lonely"

// Badge is one earn of a badge kind, with the earner and group for display.
type Badge struct {
	Token, Kind, Day     string // Day YYYY-MM-DD
	GroupID              int64
	GroupName            string
	UserID               int64
	Username, Pawn, Lang string // the earner
}

// Unseen is a member's news in one group: badges earned since they last
// opened its Badges tab.
type Unseen struct {
	Count  int
	Oldest string // YYYY-MM-DD
	Kind   string // the newest one's
}

// WallRow is one member's shelf: how many of each kind they earned.
type WallRow struct {
	Username, Pawn string
	Counts         map[string]int
	Total          int
}

// lonelyDays returns, for each day in days, the member who was the only one
// to play any puzzle that day, when at least one other member counted
// (joined[u] < day). Players who are not in joined (left the group) are
// ignored. Days without a winner are absent.
func lonelyDays(ps []puzzlePlays, joined map[int64]string, days []string) map[string]int64 {
	players := map[string]map[int64]bool{}
	for _, p := range ps {
		if !slices.Contains(days, p.day) {
			continue
		}
		for _, e := range p.entries {
			if _, member := joined[e.user]; !member {
				continue
			}
			if players[p.day] == nil {
				players[p.day] = map[int64]bool{}
			}
			players[p.day][e.user] = true
		}
	}
	out := map[string]int64{}
	for day, who := range players {
		if len(who) != 1 {
			continue
		}
		counted := 0
		for _, j := range joined {
			if j < day {
				counted++
			}
		}
		for u := range who {
			if counted >= 2 && joined[u] < day {
				out[day] = u
			}
		}
	}
	return out
}

// AwardBadges evaluates every day since the group's last evaluated one that
// is over in all its games, like seasons: on read, and only whole days, so
// when it runs changes nothing. Concurrent reads insert nothing twice.
func (s *Store) AwardBadges(groupID int64) error {
	now := s.Now()
	ids, err := s.GroupGames(groupID)
	if err != nil {
		return err
	}
	var last string
	if err := s.db.QueryRow("SELECT badges_through FROM groups WHERE id = ?", groupID).Scan(&last); err != nil {
		return err
	}
	d, err := time.Parse(time.DateOnly, last)
	if err != nil {
		return err
	}
	// never past today's UTC date: a group without games has no day "running"
	var days []string
	for d = d.AddDate(0, 0, 1); !d.After(now) && dayOver(ids, d.Format(time.DateOnly), now); d = d.AddDate(0, 0, 1) {
		days = append(days, d.Format(time.DateOnly))
	}
	if len(days) == 0 {
		return nil
	}
	ps, joined, err := s.groupPlays(groupID, days[0])
	if err != nil {
		return err
	}
	won := lonelyDays(ps, joined, days)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, day := range days {
		u, ok := won[day]
		if !ok {
			continue
		}
		token, _ := newToken() // the share link is public: stored as is, unlike sessions
		if _, err := tx.Exec(`INSERT OR IGNORE INTO badges (token, group_id, user_id, kind, day, earned_at)
			VALUES (?, ?, ?, ?, ?, ?)`, token, groupID, u, KindLonely, day, now.Unix()); err != nil {
			return err
		}
	}
	end := days[len(days)-1]
	if _, err := tx.Exec("UPDATE groups SET badges_through = ? WHERE id = ? AND badges_through < ?", end, groupID, end); err != nil {
		return err
	}
	return tx.Commit()
}

// Unseen maps each of the user's groups with news to it: their own badges
// earned after they last opened that group's Badges tab.
func (s *Store) Unseen(userID int64) (map[int64]Unseen, error) {
	rows, err := s.db.Query(`
		SELECT b.group_id, b.day, b.kind FROM badges b
		JOIN memberships m ON m.group_id = b.group_id AND m.user_id = b.user_id
		WHERE b.user_id = ? AND b.earned_at > m.badges_seen_at
		ORDER BY b.day`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]Unseen{}
	for rows.Next() {
		var g int64
		var day, kind string
		if err := rows.Scan(&g, &day, &kind); err != nil {
			return nil, err
		}
		u := out[g]
		if u.Count == 0 {
			u.Oldest = day
		}
		u.Count++
		u.Kind = kind
		out[g] = u
	}
	return out, rows.Err()
}

// MarkBadgesSeen clears the member's news in the group.
func (s *Store) MarkBadgesSeen(groupID, userID int64) error {
	_, err := s.db.Exec("UPDATE memberships SET badges_seen_at = ? WHERE group_id = ? AND user_id = ?",
		s.Now().Unix(), groupID, userID)
	return err
}

// Wall lists every current member's badge counts, in Members order.
func (s *Store) Wall(groupID int64) ([]WallRow, error) {
	members, err := s.Members(groupID)
	if err != nil {
		return nil, err
	}
	counts := map[int64]map[string]int{}
	rows, err := s.db.Query("SELECT user_id, kind, COUNT(*) FROM badges WHERE group_id = ? GROUP BY user_id, kind", groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var u int64
		var kind string
		var n int
		if err := rows.Scan(&u, &kind, &n); err != nil {
			return nil, err
		}
		if counts[u] == nil {
			counts[u] = map[string]int{}
		}
		counts[u][kind] = n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]WallRow, 0, len(members))
	for _, m := range members {
		r := WallRow{Username: m.Username, Pawn: m.Pawn, Counts: counts[m.UserID]}
		if r.Counts == nil {
			r.Counts = map[string]int{}
		}
		for _, n := range r.Counts {
			r.Total += n
		}
		out = append(out, r)
	}
	return out, nil
}

// BadgesOf lists the user's badges in the group, newest day first.
func (s *Store) BadgesOf(groupID, userID int64) ([]Badge, error) {
	rows, err := s.db.Query("SELECT token, kind, day FROM badges WHERE group_id = ? AND user_id = ? ORDER BY day DESC, kind",
		groupID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Badge
	for rows.Next() {
		b := Badge{GroupID: groupID, UserID: userID}
		if err := rows.Scan(&b.Token, &b.Kind, &b.Day); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// BadgeByToken loads a shared badge with its group and earner; ErrNotFound
// for an unknown token.
func (s *Store) BadgeByToken(token string) (Badge, error) {
	b := Badge{Token: token}
	err := s.db.QueryRow(`
		SELECT b.kind, b.day, g.id, g.name, u.id, u.username, u.pawn, u.lang
		FROM badges b JOIN groups g ON g.id = b.group_id JOIN users u ON u.id = b.user_id
		WHERE b.token = ?`, token).Scan(&b.Kind, &b.Day, &b.GroupID, &b.GroupName, &b.UserID, &b.Username, &b.Pawn, &b.Lang)
	if errors.Is(err, sql.ErrNoRows) {
		return Badge{}, ErrNotFound
	}
	return b, err
}
