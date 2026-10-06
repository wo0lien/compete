// Command compete serves the compete web app.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/wo0lien/compete/store"
	"github.com/wo0lien/compete/web"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	addr := flag.String("addr", env("COMPETE_ADDR", ":8080"), "listen address")
	db := flag.String("db", env("COMPETE_DB", "compete.db"), "SQLite database path")
	dev := flag.Bool("dev", false, "allow session cookies over plain http (local development only)")
	proxy := flag.Bool("trust-proxy", false, "take the client IP from the last X-Forwarded-For entry")
	flag.Parse()

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
		fmt.Printf("/reset/%s (valid 24h; prefix it with your instance URL)\n", tok)
		return
	}

	// No request logging on purpose: the instance keeps no IPs.
	srv := &http.Server{Addr: *addr, Handler: web.New(st, !*dev, *proxy), ReadHeaderTimeout: 10 * time.Second}
	log.Printf("compete listening on %s", *addr)
	log.Fatal(srv.ListenAndServe())
}
