package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthURL(t *testing.T) {
	for in, want := range map[string]string{
		":8080":          "http://127.0.0.1:8080/",
		"127.0.0.1:8081": "http://127.0.0.1:8081/",
		"[::1]:9000":     "http://[::1]:9000/",
	} {
		if got := healthURL(in); got != want {
			t.Errorf("healthURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHealthcheck(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusSeeOther) // anonymous visitors are redirected
	}))
	defer ok.Close()
	if err := healthcheck(ok.URL+"/", time.Second); err != nil {
		t.Errorf("redirect should be healthy: %v", err)
	}

	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer broken.Close()
	if err := healthcheck(broken.URL+"/", time.Second); err == nil {
		t.Error("500 should be unhealthy")
	}

	if err := healthcheck("http://127.0.0.1:1/", 200*time.Millisecond); err == nil {
		t.Error("unreachable should be unhealthy")
	}
}
