package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPawnColor(t *testing.T) { // Review Focus 5
	ts, _ := newTestServer(t)
	c := newClient(t)
	signup(t, ts, c, "alice")
	if resp, _ := post(t, c, ts.URL+"/account/pawn", url.Values{"pawn": {"#123abc"}}); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("set pawn = %d", resp.StatusCode)
	}
	if _, body := get(t, c, ts.URL+"/account"); !strings.Contains(body, `fill="#123abc"`) {
		t.Fatal("account page does not use the new pawn color")
	}
	if resp, _ := post(t, c, ts.URL+"/account/pawn", url.Values{"pawn": {`#000000" onload="alert(1)`}}); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("bad pawn = %d, want 422", resp.StatusCode)
	}
}

func TestDeleteAccount(t *testing.T) {
	ts, _ := newTestServer(t)
	c := newClient(t)
	signup(t, ts, c, "alice")
	if resp, _ := post(t, c, ts.URL+"/account/delete", url.Values{"password": {"nope nope nope"}}); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong password = %d, want 401", resp.StatusCode)
	}
	if resp, _ := post(t, c, ts.URL+"/account/delete", url.Values{"password": {"correct horse"}}); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("delete = %d", resp.StatusCode)
	}
	resp, _ := post(t, newClient(t), ts.URL+"/login", url.Values{"username": {"alice"}, "password": {"correct horse"}})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("deleted account can still log in: %d", resp.StatusCode)
	}
}

func TestPasswordReset(t *testing.T) {
	ts, st := newTestServer(t)
	signup(t, ts, newClient(t), "alice")
	tok, _ := st.CreateResetToken("alice", time.Hour)
	c := newClient(t)
	if resp, _ := get(t, c, ts.URL+"/reset/"+tok); resp.StatusCode != 200 {
		t.Fatalf("reset form = %d", resp.StatusCode)
	}
	resp, _ := post(t, c, ts.URL+"/reset/"+tok, url.Values{"password": {"brand new password"}})
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/login" {
		t.Fatalf("reset = %d → %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	if resp, _ := post(t, c, ts.URL+"/reset/"+tok, url.Values{"password": {"another password"}}); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("reused token = %d, want 404", resp.StatusCode)
	}
}
