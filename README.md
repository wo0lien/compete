# compete

Paste the share text of your daily games (Tusmo, Songless, Travle…) and race your friends.
Scores stay hidden until you've played; the leaderboard is a board-game race with your pawn.

- One Go binary + one SQLite file. Runs on the smallest VPS.
- No email, no tracking, no IP logs: a pseudonym and a password hash, that's it.
- Works without JavaScript; installable as an app (Android can share straight into it).

## Run locally

    go run . -dev            # http://localhost:8080

## Self-host

Docker (behind Caddy or another TLS proxy: session cookies are HTTPS-only):

    docker build -t compete .
    docker run -d -p 127.0.0.1:8080:8080 -v compete:/data compete -trust-proxy

Just trying it on your machine over plain http? Add `-dev` instead of `-trust-proxy`
and open http://localhost:8080.

Prebuilt Linux binaries (amd64, arm64) and SHA-256 checksums are attached to each
[GitHub release](https://github.com/wo0lien/compete/releases); `compete -version` prints the release tag.

Bare metal: copy the binary to `/usr/local/bin/compete`, install `deploy/compete.service`,
and put Caddy in front with `deploy/Caddyfile` (TLS is automatic). Run with `-trust-proxy`
behind a proxy so rate limiting sees real client IPs.

Backups: `sqlite3 /var/lib/compete/compete.db ".backup /backups/compete-$(date +%F).db"` from cron.

Forgotten password: `compete -db /var/lib/compete/compete.db reset-link <username>` prints a
one-time link (24 h) to hand to the user.

Invite links use the request's host and `reset-link` prints a bare path; set
`-base-url https://compete.example.org` (or `COMPETE_BASE_URL`) to use your public URL in both.

## Add a game

1. `games/<game>.go`: a `Game` value with a parse func, registered in `games.All`.
2. `games/testdata/<game>/`: real share texts, at least a win and a fail.
3. A test using `checkFixture`. CI must pass.

Can't code? [Request a game](https://github.com/wo0lien/compete/issues/new?template=request-game.md).

## Licenses

Fonts in `web/static/fonts/` are under the SIL Open Font License (see the OFL files there).
htmx is BSD-licensed.
