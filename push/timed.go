package push

import (
	"log"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/wo0lien/compete/games"
)

// The digest and the reminder follow Paris time, where the group lives.
// ponytail: one timezone for everyone, like Travle's reset.
var parisLoc, _ = time.LoadLocation("Europe/Paris")

// Run ticks every minute until the process exits.
func (n *Notifier) Run() {
	for now := range time.Tick(time.Minute) {
		n.Tick(now)
	}
}

// Tick sends the morning digest (08:00–10:00) or the evening reminder
// (20:00–22:00) to members who still have games to play today. Each is sent at
// most once per member and day; outside the windows nothing goes out, so a
// restart never pings at night.
func (n *Notifier) Tick(now time.Time) {
	local := now.In(parisLoc)
	switch h := local.Hour(); {
	case h >= 8 && h < 10:
		n.timed("digest", local)
	case h >= 20 && h < 22:
		n.timed("reminder", local)
	}
	if err := n.Store.PruneSent(now.Add(-7 * 24 * time.Hour)); err != nil {
		log.Printf("push prune: %v", err)
	}
}

func (n *Notifier) timed(kind string, local time.Time) {
	ids, err := n.Store.PushUsers()
	if err != nil {
		log.Printf("push %s: %v", kind, err)
		return
	}
	for _, id := range ids {
		r, err := n.Store.Recipient(id)
		if err != nil {
			log.Printf("push %s: %v", kind, err)
			continue
		}
		if (kind == "digest" && !r.Prefs.Digest) || (kind == "reminder" && !r.Prefs.Reminder) {
			continue
		}
		todo, err := n.Store.ToPlay(id)
		if err != nil || len(todo) == 0 {
			continue
		}
		// at most once: recorded before delivery, so a crash skips rather than repeats
		if first, err := n.Store.MarkSent(id, kind+":"+local.Format(time.DateOnly)); err != nil || !first {
			continue
		}
		var names []string
		for _, g := range todo {
			game, _ := games.ByID(g)
			names = append(names, game.Name)
		}
		l := lang(r)
		n.sendAll(r, Message{Title: "compete", Body: n.Text(l, "push."+kind, "Games", strings.Join(names, ", ")), URL: "/"})
	}
}
