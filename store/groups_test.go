package store

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestCreateAndJoin(t *testing.T) {
	s := newStore(t)
	alice, bob := mustUser(t, s, "alice"), mustUser(t, s, "bob")
	g, err := s.CreateGroup(alice.ID, "  Friends ")
	if err != nil {
		t.Fatal(err)
	}
	if g.Name != "Friends" || g.Role != "owner" || len(g.InviteCode) != 10 {
		t.Fatalf("CreateGroup = %+v", g)
	}
	if byCode, err := s.GroupByCode(g.InviteCode); err != nil || byCode.ID != g.ID {
		t.Fatalf("GroupByCode = %+v, %v", byCode, err)
	}
	for range 2 { // joining twice is a no-op
		j, err := s.JoinByCode(bob.ID, strings.ToUpper(g.InviteCode))
		if err != nil || j.ID != g.ID || j.Role != "member" {
			t.Fatalf("JoinByCode = %+v, %v", j, err)
		}
	}
	ms, _ := s.Members(g.ID)
	if len(ms) != 2 || ms[0].Username != "alice" || ms[0].Role != "owner" || ms[1].Username != "bob" {
		t.Fatalf("Members = %+v", ms)
	}
	if gs, _ := s.GroupsOf(bob.ID); len(gs) != 1 || gs[0].ID != g.ID {
		t.Fatalf("GroupsOf(bob) = %+v", gs)
	}
}

func TestOutsidersSeeNothing(t *testing.T) {
	s := newStore(t)
	alice, carol := mustUser(t, s, "alice"), mustUser(t, s, "carol")
	g, _ := s.CreateGroup(alice.ID, "Friends")
	if _, err := s.GroupFor(carol.ID, g.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GroupFor(non-member) err = %v, want ErrNotFound", err)
	}
	if _, err := s.JoinByCode(carol.ID, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("JoinByCode(bad code) err = %v, want ErrNotFound", err)
	}
}

func TestRegenerateCodeKillsOldLink(t *testing.T) {
	s := newStore(t)
	alice, bob := mustUser(t, s, "alice"), mustUser(t, s, "bob")
	g, _ := s.CreateGroup(alice.ID, "Friends")
	code, err := s.RegenerateCode(g.ID)
	if err != nil || code == g.InviteCode {
		t.Fatalf("RegenerateCode = %q, %v", code, err)
	}
	if _, err := s.JoinByCode(bob.ID, g.InviteCode); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old code err = %v, want ErrNotFound", err)
	}
	if _, err := s.JoinByCode(bob.ID, code); err != nil {
		t.Fatalf("new code: %v", err)
	}
}

func TestRemoveMemberNeverRemovesOwner(t *testing.T) {
	s := newStore(t)
	alice, bob := mustUser(t, s, "alice"), mustUser(t, s, "bob")
	g, _ := s.CreateGroup(alice.ID, "Friends")
	s.JoinByCode(bob.ID, g.InviteCode)
	if err := s.RemoveMember(g.ID, bob.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GroupFor(bob.ID, g.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("kicked member still in group: %v", err)
	}
	s.RemoveMember(g.ID, alice.ID)
	if _, err := s.GroupFor(alice.ID, g.ID); err != nil {
		t.Fatalf("owner was removed: %v", err)
	}
}

func TestRenameAndDelete(t *testing.T) {
	s := newStore(t)
	alice := mustUser(t, s, "alice")
	g, _ := s.CreateGroup(alice.ID, "Friends")
	if err := s.RenameGroup(g.ID, "   "); !errors.Is(err, ErrBadGroupName) {
		t.Fatalf("blank rename err = %v, want ErrBadGroupName", err)
	}
	s.RenameGroup(g.ID, "Family")
	if got, _ := s.GroupFor(alice.ID, g.ID); got.Name != "Family" {
		t.Fatalf("after rename: %+v", got)
	}
	if err := s.DeleteGroup(g.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GroupFor(alice.ID, g.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted group still visible: %v", err)
	}
}

func TestDeleteUserDeletesOnlyOwnedGroups(t *testing.T) {
	s := newStore(t)
	alice, bob := mustUser(t, s, "alice"), mustUser(t, s, "bob")
	owned, _ := s.CreateGroup(alice.ID, "Alice's")
	other, _ := s.CreateGroup(bob.ID, "Bob's")
	s.JoinByCode(bob.ID, owned.InviteCode)
	s.JoinByCode(alice.ID, other.InviteCode)
	if err := s.DeleteUser(alice.ID); err != nil {
		t.Fatal(err)
	}
	if gs, _ := s.GroupsOf(bob.ID); len(gs) != 1 || gs[0].ID != other.ID {
		t.Fatalf("bob's groups = %+v, want only %q", gs, other.Name)
	}
	if ms, _ := s.Members(other.ID); len(ms) != 1 {
		t.Fatalf("members of %q = %+v, want bob only", other.Name, ms)
	}
}

func TestGroupCap(t *testing.T) {
	s := newStore(t)
	alice, bob := mustUser(t, s, "alice"), mustUser(t, s, "bob")
	for i := range MaxGroupsPerUser {
		if _, err := s.CreateGroup(alice.ID, fmt.Sprint("g", i)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.CreateGroup(alice.ID, "one more"); !errors.Is(err, ErrTooManyGroups) {
		t.Fatalf("create over cap err = %v", err)
	}
	g, _ := s.CreateGroup(bob.ID, "Bob's")
	if _, err := s.JoinByCode(alice.ID, g.InviteCode); !errors.Is(err, ErrTooManyGroups) {
		t.Fatalf("join over cap err = %v", err)
	}
}

// Several friends tapping the invite link at once must all get in: read-then-write
// transactions must not fail with "database is locked" (SQLITE_BUSY upgrade).
func TestConcurrentJoins(t *testing.T) {
	s := newStore(t)
	owner := mustUser(t, s, "owner")
	g, _ := s.CreateGroup(owner.ID, "Friends")
	var users []User
	for i := range 15 {
		users = append(users, mustUser(t, s, fmt.Sprint("user", i)))
	}
	errs := make(chan error, len(users))
	for _, u := range users {
		go func() {
			_, err := s.JoinByCode(u.ID, g.InviteCode)
			errs <- err
		}()
	}
	for range users {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
}
