package web

import (
	"cmp"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/wo0lien/compete/store"
)

// badgeKind is how a badge kind looks; its name, quote and quip are the
// messages badge.<ID>.title, .quote and .quip. Its link preview picture is
// badges/<ID>-og-<lang>.png, one per UI language.
type badgeKind struct {
	ID, Image string // Image: static path, for asset()
}

var badgeKinds = []badgeKind{{store.KindLonely, "badges/lonely.svg"}}

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

// sharePage is a badge's public page (/b/{token}): the brag link. It needs
// no login, since chat apps fetch it for their preview without cookies; the
// preview tags speak the earner's language, the page the viewer's.
func (s *Server) sharePage(w http.ResponseWriter, r *http.Request) {
	b, err := s.store.BadgeByToken(r.PathValue("token"))
	if errors.Is(err, store.ErrNotFound) {
		s.message(w, r, http.StatusNotFound, "msg.not_found", "badges.not_found")
		return
	}
	if err != nil {
		s.oops(w, r, err)
		return
	}
	data := map[string]any{"Badge": b, "Next": "/b/" + b.Token}
	if u, ok := userFrom(r); ok {
		_, err := s.store.GroupFor(u.ID, b.GroupID)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			s.oops(w, r, err)
			return
		}
		data["Member"] = err == nil
	}
	lang, base := cmp.Or(b.Lang, langs[0].String()), s.baseURL(r)
	title := tr(lang, "badge."+b.Kind+".title")
	data["OG"] = map[string]string{
		"Title":       tr(lang, "badges.og_title", "Name", b.Username, "Title", title),
		"Description": tr(lang, "badge."+b.Kind+".quip", "Group", b.GroupName, "Day", shortDate(lang, b.Day)),
		"Image":       base + "/static/badges/" + kindOf(b.Kind).ID + "-og-" + lang + ".png",
		"URL":         base + "/b/" + b.Token,
	}
	s.render(w, r, http.StatusOK, "share.html", data)
}

// news is a member's unseen badges in a group, with the minis to draw: up
// to max medallions, then "+More".
type news struct {
	store.Unseen
	Minis []string // kinds, one per medallion
	More  int
}

func newsOf(u store.Unseen, max int) news {
	n := news{Unseen: u}
	for range min(u.Count, max) {
		n.Minis = append(n.Minis, u.Kind)
	}
	n.More = u.Count - len(n.Minis)
	return n
}

// awardNews awards the groups' due badges and returns the user's news per
// group. Badges are a bonus: a failure is logged and the page renders without
// them.
func (s *Server) awardNews(u store.User, groups []int64, max int) map[int64]news {
	for _, g := range groups {
		if err := s.store.AwardBadges(g); err != nil {
			log.Printf("badges: group %d: %v", g, err)
		}
	}
	unseen, err := s.store.Unseen(u.ID)
	if err != nil {
		log.Printf("badges: unseen for user %d: %v", u.ID, err)
	}
	out := map[int64]news{}
	for g, n := range unseen {
		out[g] = newsOf(n, max)
	}
	return out
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
