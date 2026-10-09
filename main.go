// Command compete serves the compete web app.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/wo0lien/compete/store"
	"github.com/wo0lien/compete/web"
)

// version is set by release builds: -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// healthURL turns a listen address into the URL a local health check should hit.
func healthURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return "http://" + host + ":" + port + "/"
}

// healthcheck reports whether the app answers with a 2xx or 3xx (redirects are not followed).
func healthcheck(url string, timeout time.Duration) error {
	c := &http.Client{
		Timeout:       timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := c.Get(url)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	return nil
}

func main() {
	addr := flag.String("addr", env("COMPETE_ADDR", ":8080"), "listen address")
	db := flag.String("db", env("COMPETE_DB", "compete.db"), "SQLite database path")
	dev := flag.Bool("dev", false, "allow session cookies over plain http (local development only)")
	proxy := flag.Bool("trust-proxy", false, "take the client IP from the last X-Forwarded-For entry")
	baseURL := flag.String("base-url", env("COMPETE_BASE_URL", ""), "public URL for invite and reset links, e.g. https://play.example.org (default: the request's host)")
	showVersion := flag.Bool("version", false, "print the version and exit")
	health := flag.Bool("healthcheck", false, "check that the instance at -addr answers, exit 0 or 1 (for container health checks)")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	if *health {
		if err := healthcheck(healthURL(*addr), 2*time.Second); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	st, err := store.Open(*db)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	if flag.Arg(0) == "reset-link" {
		// No email on purpose: the admin hands this link to the user.
		tok, err := st.CreateResetToken(flag.Arg(1), 24*time.Hour)
		if err != nil {
			log.Fatalf("reset-link %q: %v", flag.Arg(1), err)
		}
		if *baseURL != "" {
			fmt.Printf("%s/reset/%s (valid 24h)\n", strings.TrimSuffix(*baseURL, "/"), tok)
		} else {
			fmt.Printf("/reset/%s (valid 24h; prefix it with your instance URL, or set -base-url)\n", tok)
		}
		return
	}

	// No request logging on purpose: the instance keeps no IPs.
	h := web.New(st, !*dev, *proxy)
	h.BaseURL = strings.TrimSuffix(*baseURL, "/")
	h.Version = version
	srv := &http.Server{Addr: *addr, Handler: h, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("compete listening on %s", *addr)
	log.Fatal(srv.ListenAndServe())
}
