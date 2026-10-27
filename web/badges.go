package web

import (
	"net/http"
	"strings"
	"time"

	"github.com/wo0lien/compete/store"
)

// badgeKind is how a badge kind looks; its title and quip are the messages
// badge.<ID>.title and badge.<ID>.quip.
type badgeKind struct {
	ID, Image, OGImage string // static paths, for asset()
}

var badgeKinds = []badgeKind{{store.KindLonely, "badges/lonely.svg", "badges/lonely-og.png"}}

func kindOf(id string) badgeKind {
	for _, k := range badgeKinds {
		if k.ID == id {
			return k
		}
	}
	return badgeKinds[0]
}

// shortDate renders a YYYY-MM-DD day as "Mon 26 Oct" / "lun. 26 oct.".
func shortDate(lang, day string) string {
	d, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return day
	}
	return strings.Fields(tr(lang, "badges.weekdays"))[d.Weekday()] + " " +
		d.Format("2") + " " + strings.Fields(tr(lang, "history.months"))[d.Month()-1]
}

// baseURL is the app's public origin for absolute links: -base-url, else the
// request's host.
func (s *Server) baseURL(r *http.Request) string {
	if s.BaseURL != "" {
		return s.BaseURL
	}
	if s.secure {
		return "https://" + r.Host
	}
	return "http://" + r.Host
}

// shelf is one slot on a member's wall: a kind and how often they earned it.
type shelf struct {
	Kind string
	N    int
}

type wallRow struct {
	store.WallRow
	Shelf []shelf
}

// badgesPage is the group's Badges tab: everyone's wall and the viewer's own
// badges to share. Opening it clears the viewer's news in this group.
func (s *Server) badgesPage(w http.ResponseWriter, r *http.Request, u store.User) {
	g, ok := s.groupFor(w, r, u)
	if !ok {
		return
	}
	if err := s.store.AwardBadges(g.ID); err != nil {
		s.oops(w, r, err)
		return
	}
	if err := s.store.MarkBadgesSeen(g.ID, u.ID); err != nil {
		s.oops(w, r, err)
		return
	}
	wall, err := s.store.Wall(g.ID)
	if err != nil {
		s.oops(w, r, err)
		return
	}
	mine, err := s.store.BadgesOf(g.ID, u.ID)
	if err != nil {
		s.oops(w, r, err)
		return
	}
	rows := make([]wallRow, len(wall))
	for i, m := range wall {
		rows[i] = wallRow{WallRow: m}
		for _, k := range badgeKinds {
			rows[i].Shelf = append(rows[i].Shelf, shelf{k.ID, m.Counts[k.ID]})
		}
	}
	s.render(w, r, http.StatusOK, "badges.html", map[string]any{
		"Group": g, "Tab": "badges", "Wall": rows, "Mine": mine, "Base": s.baseURL(r),
	})
}
