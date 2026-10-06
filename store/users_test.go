package store

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func mustUser(t *testing.T, s *Store, name string) User {
	t.Helper()
	u, err := s.CreateUser(name, "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestCreateAndAuthenticate(t *testing.T) {
	s := newStore(t)
	u := mustUser(t, s, "alice")
	got, err := s.Authenticate("ALICE", "correct horse") // usernames are case-insensitive
	if err != nil || got.ID != u.ID {
		t.Fatalf("Authenticate = %+v, %v; want %+v", got, err, u)
	}
	if _, err := s.Authenticate("alice", "wrong password"); !errors.Is(err, ErrBadLogin) {
		t.Fatalf("wrong password: err = %v, want ErrBadLogin", err)
	}
	if _, err := s.Authenticate("bob", "correct horse"); !errors.Is(err, ErrBadLogin) {
		t.Fatalf("unknown user: err = %v, want ErrBadLogin", err)
	}
}

func TestCreateUserValidation(t *testing.T) {
	s := newStore(t)
	mustUser(t, s, "alice")
	cases := []struct {
		name, user, pw string
		want           error
	}{
		{"taken in other case", "Alice", "correct horse", ErrUsernameTaken},
		{"too short", "al", "correct horse", ErrBadUsername},
		{"space", "al ice", "correct horse", ErrBadUsername},
		{"weak password", "bob", "short", ErrWeakPassword},
	}
	for _, c := range cases {
		if _, err := s.CreateUser(c.user, c.pw); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
}

func TestSessions(t *testing.T) {
	s := newStore(t)
	u := mustUser(t, s, "alice")
	tok, err := s.CreateSession(u.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.UserBySession(tok); err != nil || got != u {
		t.Fatalf("UserBySession = %+v, %v; want %+v", got, err, u)
	}
	if err := s.DeleteSession(tok); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserBySession(tok); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: err = %v, want ErrNotFound", err)
	}
	expired, _ := s.CreateSession(u.ID, -time.Second)
	if _, err := s.UserBySession(expired); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired: err = %v, want ErrNotFound", err)
	}
}

func TestResetPassword(t *testing.T) {
	s := newStore(t)
	u := mustUser(t, s, "alice")
	session, _ := s.CreateSession(u.ID, time.Hour)
	tok, err := s.CreateResetToken("alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ResetPassword(tok, "new password 1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate("alice", "new password 1"); err != nil {
		t.Fatalf("login with new password: %v", err)
	}
	if _, err := s.UserBySession(session); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old session survived reset: err = %v", err)
	}
	if err := s.ResetPassword(tok, "another password"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("reused token: err = %v, want ErrNotFound", err)
	}
	if _, err := s.CreateResetToken("nobody", time.Hour); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown user: err = %v, want ErrNotFound", err)
	}
}

func TestDeleteUserRemovesSessions(t *testing.T) {
	s := newStore(t)
	u := mustUser(t, s, "alice")
	tok, _ := s.CreateSession(u.ID, time.Hour)
	if err := s.DeleteUser(u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserBySession(tok); !errors.Is(err, ErrNotFound) {
		t.Fatalf("session survived deletion: err = %v", err)
	}
	if _, err := s.Authenticate("alice", "correct horse"); !errors.Is(err, ErrBadLogin) {
		t.Fatalf("deleted user can log in: err = %v", err)
	}
}

func TestPawn(t *testing.T) {
	s := newStore(t)
	u := mustUser(t, s, "alice")
	if !slices.Contains(PawnPalette, u.Pawn) {
		t.Fatalf("default pawn %q not in palette", u.Pawn)
	}
	if err := s.SetPawn(u.ID, "#A1B2C3"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Authenticate("alice", "correct horse"); got.Pawn != "#a1b2c3" {
		t.Fatalf("pawn = %q, want #a1b2c3", got.Pawn)
	}
	for _, bad := range []string{"red", "#fff", `#123456" onload="x`, "#12345g"} {
		if err := s.SetPawn(u.ID, bad); !errors.Is(err, ErrBadPawn) {
			t.Errorf("SetPawn(%q) err = %v, want ErrBadPawn", bad, err)
		}
	}
}
