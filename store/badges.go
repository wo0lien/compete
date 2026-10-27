package store

import "slices"

// lonelyDays returns, for each day in days, the member who was the only one
// to play any puzzle that day, when at least one other member counted
// (joined[u] < day). Players who are not in joined (left the group) are
// ignored. Days without a winner are absent.
func lonelyDays(ps []puzzlePlays, joined map[int64]string, days []string) map[string]int64 {
	players := map[string]map[int64]bool{}
	for _, p := range ps {
		if !slices.Contains(days, p.day) {
			continue
		}
		for _, e := range p.entries {
			if _, member := joined[e.user]; !member {
				continue
			}
			if players[p.day] == nil {
				players[p.day] = map[int64]bool{}
			}
			players[p.day][e.user] = true
		}
	}
	out := map[string]int64{}
	for day, who := range players {
		if len(who) != 1 {
			continue
		}
		counted := 0
		for _, j := range joined {
			if j < day {
				counted++
			}
		}
		for u := range who {
			if counted >= 2 && joined[u] < day {
				out[day] = u
			}
		}
	}
	return out
}
