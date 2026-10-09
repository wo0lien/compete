package push

import (
	"fmt"
	"log"
	"strings"

	"github.com/wo0lien/compete/games"
)

// Submitted pings the submitter's groups after new results: members the
// submitter just beat, everyone once the whole group has played, or (opt-in)
// that a friend played. One push per member per group, strongest kind first;
// a score only goes to members who played that puzzle.
func (n *Notifier) Submitted(userID int64, added []games.Result) {
	groups, err := n.Store.GroupsOf(userID)
	if err != nil {
		log.Printf("push submitted: %v", err)
		return
	}
	for _, g := range groups {
		members, err := n.Store.Members(g.ID)
		if err != nil {
			log.Printf("push submitted: %v", err)
			continue
		}
		ids := map[string]int64{}
		var name string
		for _, m := range members {
			ids[m.Username] = m.UserID
			if m.UserID == userID {
				name = m.Username
			}
		}
		// best[uid] is the strongest message for that member in this group: 3 overtaken, 2 everyone, 1 friend
		type pick struct {
			rank int
			kind string
			args []any
		}
		best := map[int64]pick{}
		offer := func(uid int64, p pick) {
			if uid != userID && p.rank > best[uid].rank {
				best[uid] = p
			}
		}
		for _, r := range added {
			b, err := n.Store.Board(g.ID, userID, r.Game, r.Variant, r.PuzzleID)
			if err != nil {
				log.Printf("push submitted: %v", err)
				continue
			}
			game, _ := games.ByID(r.Game)
			board := strings.TrimSpace(fmt.Sprintf("%s #%d %s", game.Name, r.PuzzleID, r.Variant))
			myRank, all := 0, true
			for _, e := range b.Entries {
				if e.Me {
					myRank = e.Rank
				}
				all = all && e.Played
			}
			for _, e := range b.Entries {
				uid := ids[e.Username]
				if e.Played && e.Rank > myRank {
					offer(uid, pick{3, "overtaken", []any{"Name", name, "Board", board, "Score", game.ShowScore(r.Score)}})
				}
				if all {
					offer(uid, pick{2, "everyone", []any{"Group", g.Name, "Name", name, "Board", board}})
				}
				offer(uid, pick{1, "friend", []any{"Name", name, "Board", board}})
			}
		}
		for uid, p := range best {
			n.sendSocial(uid, g.ID, p.kind, p.args)
		}
	}
}

// sendSocial delivers a social ping if the member wants that kind, falling
// back to a friend ping when social pings are off but friend pings are on.
func (n *Notifier) sendSocial(uid, groupID int64, kind string, args []any) {
	r, err := n.Store.Recipient(uid)
	if err != nil || len(r.Subs) == 0 {
		return
	}
	if kind != "friend" && !r.Prefs.Social {
		if !r.Prefs.Friends {
			return
		}
		kind, args = "friend", friendArgs(args)
	}
	if kind == "friend" && !r.Prefs.Friends {
		return
	}
	n.sendAll(r, Message{Title: "compete", Body: n.Text(lang(r), "push."+kind, args...), URL: fmt.Sprintf("/g/%d", groupID)})
}

// friendArgs keeps Name and Board (no score) from a stronger ping's args.
func friendArgs(args []any) []any {
	var out []any
	for i := 0; i+1 < len(args); i += 2 {
		if k := args[i]; k == "Name" || k == "Board" {
			out = append(out, args[i], args[i+1])
		}
	}
	return out
}
