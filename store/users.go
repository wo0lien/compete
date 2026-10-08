package store

import (
	"database/sql"
	"errors"
	"hash/fnv"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrUsernameTaken = errors.New("username already taken")
	ErrBadUsername   = errors.New("username must be 3-24 letters, digits, _ or -")
	ErrWeakPassword  = errors.New("password must be at least 10 characters")
	ErrBadLogin      = errors.New("wrong username or password")
)

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9_-]{3,24}$`)

var ErrBadPawn = errors.New("pawn color must look like #1a2b3c")

var pawnRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// PawnPalette gives new users a distinct default pawn color.
var PawnPalette = []string{"#ff6b5b", "#9b6bff", "#ff9f43", "#5ed67e", "#ffd54a", "#ff7eb6", "#2b8ec2", "#10264a"}

func defaultPawn(username string) string {
	h := fnv.New32a()
	h.Write([]byte(strings.ToLower(username)))
	return PawnPalette[h.Sum32()%uint32(len(PawnPalette))]
}

func (s *Store) SetPawn(userID int64, color string) error {
	if !pawnRe.MatchString(color) {
		return ErrBadPawn
	}
	_, err := s.db.Exec("UPDATE users SET pawn = ? WHERE id = ?", strings.ToLower(color), userID)
	return err
}

type User struct {
	ID       int64
	Username string
	Pawn     string // #rrggbb
	Lang     string // "fr", "en", or "" to follow the browser
}

// SetLang saves the user's UI language; the web layer validates it.
func (s *Store) SetLang(userID int64, lang string) error {
	_, err := s.db.Exec("UPDATE users SET lang = ? WHERE id = ?", lang, userID)
	return err
}

func validPassword(pw string) error {
	if utf8.RuneCountInString(pw) < 10 {
		return ErrWeakPassword
	}
	return nil
}

func (s *Store) CreateUser(username, password string) (User, error) {
	if !usernameRe.MatchString(username) {
		return User{}, ErrBadUsername
	}
	if err := validPassword(password); err != nil {
		return User{}, err
	}
	pawn := defaultPawn(username)
	res, err := s.db.Exec("INSERT INTO users(username, pass_hash, pawn) VALUES (?, ?, ?)", username, hashPassword(password), pawn)
	if isUnique(err) {
		return User{}, ErrUsernameTaken
	}
	if err != nil {
		return User{}, err
	}
	id, err := res.LastInsertId()
	return User{ID: id, Username: username, Pawn: pawn}, err
}

// Authenticate returns ErrBadLogin for both unknown users and wrong passwords.
func (s *Store) Authenticate(username, password string) (User, error) {
	var u User
	var hash string
	err := s.db.QueryRow("SELECT id, username, pawn, lang, pass_hash FROM users WHERE username = ?", username).
		Scan(&u.ID, &u.Username, &u.Pawn, &u.Lang, &hash)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !checkPassword(hash, password)) {
		return User{}, ErrBadLogin
	}
	return u, err
}

// CreateSession returns a token for the cookie; only its hash is stored.
// Expired sessions are swept here, so the table can't grow unbounded.
func (s *Store) CreateSession(userID int64, ttl time.Duration) (string, error) {
	if _, err := s.db.Exec("DELETE FROM sessions WHERE expires_at <= unixepoch()"); err != nil {
		return "", err
	}
	token, hash := newToken()
	_, err := s.db.Exec("INSERT INTO sessions(token_hash, user_id, expires_at) VALUES (?, ?, ?)",
		hash, userID, time.Now().Add(ttl).Unix())
	return token, err
}

func (s *Store) UserBySession(token string) (User, error) {
	var u User
	err := s.db.QueryRow(`
		SELECT u.id, u.username, u.pawn, u.lang FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.expires_at > unixepoch()`, tokenHash(token)).Scan(&u.ID, &u.Username, &u.Pawn, &u.Lang)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE token_hash = ?", tokenHash(token))
	return err
}

// CreateResetToken is for the instance admin: there is no email, so the admin
// hands the resulting link to the user.
func (s *Store) CreateResetToken(username string, ttl time.Duration) (string, error) {
	token, hash := newToken()
	res, err := s.db.Exec(`
		INSERT INTO password_resets(token_hash, user_id, expires_at)
		SELECT ?, id, ? FROM users WHERE username = ?`, hash, time.Now().Add(ttl).Unix(), username)
	if err != nil {
		return "", err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return "", ErrNotFound
	}
	return token, nil
}

// ResetPassword consumes a reset token, sets the new password and logs out every session.
func (s *Store) ResetPassword(token, password string) error {
	if err := validPassword(password); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var uid int64
	err = tx.QueryRow(`DELETE FROM password_resets WHERE token_hash = ? AND expires_at > unixepoch() RETURNING user_id`,
		tokenHash(token)).Scan(&uid)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE users SET pass_hash = ? WHERE id = ?", hashPassword(password), uid); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM sessions WHERE user_id = ?", uid); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteUser removes the account and everything it owns. Groups it owns go too:
// there is no ownership transfer, so an owner is always the group's only owner.
func (s *Store) DeleteUser(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM groups WHERE id IN
		(SELECT group_id FROM memberships WHERE user_id = ? AND role = 'owner')`, id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM users WHERE id = ?", id); err != nil {
		return err
	}
	return tx.Commit()
}
