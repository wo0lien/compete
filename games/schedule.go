package games

import (
	"time"
	_ "time/tzdata" // the container image has no zoneinfo
)

// dayNumber numbers a daily puzzle: #1 on start (YYYY-MM-DD), one more at each
// midnight in loc.
func dayNumber(now time.Time, loc *time.Location, start string) int {
	y, m, d := now.In(loc).Date()
	s, err := time.Parse(time.DateOnly, start)
	if err != nil {
		panic(err)
	}
	return int(time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Sub(s).Hours()/24) + 1
}

// daily is a schedule whose variants all share one numbering.
func daily(tz, start string) func(string, time.Time) (int, bool) {
	loc := mustLoad(tz)
	return func(_ string, now time.Time) (int, bool) { return dayNumber(now, loc, start), true }
}

func mustLoad(tz string) *time.Location {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		panic(err)
	}
	return loc
}
