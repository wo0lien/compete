package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxGroupsPerUser caps memberships so one account can't bloat a public instance.
const MaxGroupsPerUser = 20

var (
	ErrBadGroupName  = errors.New("group name must be 1-50 characters")
	ErrTooManyGroups = fmt.Errorf("you can be in at most %d groups", MaxGroupsPerUser)
)

type Group struct {
	ID         int64
	Name       string
	InviteCode string
	Role       string // the viewer's role: "owner" or "member"
}

type Member struct {
	UserID   int64
	Username string
	Pawn     string
	Role     string
}

func newInviteCode() string {
	b := make([]byte, 8)
	rand.Read(b)
	return strings.ToLower(base32.StdEncoding.EncodeToString(b))[:10]
}

func cleanGroupName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 50 {
		return "", ErrBadGroupName
	}
	return name, nil
}

func checkGroupCap(tx *sql.Tx, userID int64) error {
	var n int
	if err := tx.QueryRow("SELECT COUNT(*) FROM memberships WHERE user_id = ?", userID).Scan(&n); err != nil {
		return err
	}
	if n >= MaxGroupsPerUser {
		return ErrTooManyGroups
	}
	return nil
}

func (s *Store) CreateGroup(ownerID int64, name string) (Group, error) {
	name, err := cleanGroupName(name)
	if err != nil {
		return Group{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Group{}, err
	}
	defer tx.Rollback()
	if err := checkGroupCap(tx, ownerID); err != nil {
		return Group{}, err
	}
	g := Group{Name: name, InviteCode: newInviteCode(), Role: "owner"}
	res, err := tx.Exec("INSERT INTO groups(name, invite_code) VALUES (?, ?)", g.Name, g.InviteCode)
	if err != nil {
		return Group{}, err
	}
	if g.ID, err = res.LastInsertId(); err != nil {
		return Group{}, err
	}
	if _, err := tx.Exec("INSERT INTO memberships(group_id, user_id, role) VALUES (?, ?, 'owner')", g.ID, ownerID); err != nil {
		return Group{}, err
	}
	return g, tx.Commit()
}

// GroupByCode looks up an invite without joining, for the "Join X?" confirm page.
func (s *Store) GroupByCode(code string) (Group, error) {
	var g Group
	err := s.db.QueryRow("SELECT id, name, invite_code FROM groups WHERE invite_code = ?",
		strings.ToLower(strings.TrimSpace(code))).Scan(&g.ID, &g.Name, &g.InviteCode)
	if errors.Is(err, sql.ErrNoRows) {
		return Group{}, ErrNotFound
	}
	return g, err
}

// JoinByCode adds the user to the group behind code. Joining twice is a no-op.
func (s *Store) JoinByCode(userID int64, code string) (Group, error) {
	g, err := s.GroupByCode(code)
	if err != nil {
		return Group{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Group{}, err
	}
	defer tx.Rollback()
	var member bool
	if err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM memberships WHERE group_id = ? AND user_id = ?)",
		g.ID, userID).Scan(&member); err != nil {
		return Group{}, err
	}
	if !member {
		if err := checkGroupCap(tx, userID); err != nil {
			return Group{}, err
		}
		if _, err := tx.Exec("INSERT INTO memberships(group_id, user_id, role) VALUES (?, ?, 'member')", g.ID, userID); err != nil {
			return Group{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Group{}, err
	}
	return s.GroupFor(userID, g.ID)
}

// GroupFor returns the group with the viewer's role, or ErrNotFound if userID is not a member.
func (s *Store) GroupFor(userID, groupID int64) (Group, error) {
	var g Group
	err := s.db.QueryRow(`
		SELECT g.id, g.name, g.invite_code, m.role FROM groups g
		JOIN memberships m ON m.group_id = g.id AND m.user_id = ?
		WHERE g.id = ?`, userID, groupID).Scan(&g.ID, &g.Name, &g.InviteCode, &g.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return Group{}, ErrNotFound
	}
	return g, err
}

func (s *Store) GroupsOf(userID int64) ([]Group, error) {
	rows, err := s.db.Query(`
		SELECT g.id, g.name, g.invite_code, m.role FROM groups g
		JOIN memberships m ON m.group_id = g.id AND m.user_id = ?
		ORDER BY g.name COLLATE NOCASE`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var gs []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.InviteCode, &g.Role); err != nil {
			return nil, err
		}
		gs = append(gs, g)
	}
	return gs, rows.Err()
}

func (s *Store) Members(groupID int64) ([]Member, error) {
	rows, err := s.db.Query(`
		SELECT u.id, u.username, u.pawn, m.role FROM memberships m JOIN users u ON u.id = m.user_id
		WHERE m.group_id = ?
		ORDER BY m.role = 'member', u.username COLLATE NOCASE`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ms []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.Username, &m.Pawn, &m.Role); err != nil {
			return nil, err
		}
		ms = append(ms, m)
	}
	return ms, rows.Err()
}

func (s *Store) RenameGroup(groupID int64, name string) error {
	name, err := cleanGroupName(name)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("UPDATE groups SET name = ? WHERE id = ?", name, groupID)
	return err
}

// RegenerateCode replaces the invite code; the old link stops working.
func (s *Store) RegenerateCode(groupID int64) (string, error) {
	code := newInviteCode()
	_, err := s.db.Exec("UPDATE groups SET invite_code = ? WHERE id = ?", code, groupID)
	return code, err
}

// RemoveMember kicks a member or lets them leave. Owners are never removed;
// they delete the group instead.
func (s *Store) RemoveMember(groupID, userID int64) error {
	_, err := s.db.Exec("DELETE FROM memberships WHERE group_id = ? AND user_id = ? AND role = 'member'", groupID, userID)
	return err
}

func (s *Store) DeleteGroup(groupID int64) error {
	_, err := s.db.Exec("DELETE FROM groups WHERE id = ?", groupID)
	return err
}
