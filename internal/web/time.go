package web

import (
	"fmt"
	"time"
)

const (
	day   = 24 * time.Hour
	month = 30 * day
	year  = 12 * month
)

var durationUnits = []struct {
	size time.Duration
	name string
}{
	{year, "year"},
	{month, "month"},
	{day, "day"},
	{time.Hour, "hour"},
	{time.Minute, "minute"},
	{time.Second, "second"},
}

// DurationFormat formats d in its largest whole unit, rounded down: "1 hour",
// "3 days" (a month is 30 days, a year 12 months). Negative durations count
// as zero.
func DurationFormat(d time.Duration) string {
	d = max(d, 0)
	for _, u := range durationUnits {
		if d >= u.size || u.size == time.Second {
			n := int64(d / u.size)
			if n == 1 {
				return fmt.Sprintf("%d %s", n, u.name)
			}
			return fmt.Sprintf("%d %ss", n, u.name)
		}
	}
	panic("unreachable")
}

// RelativeTime formats how long ago t was, like "3 days ago". A time in the
// future (clock skew between machines) shows as "just now".
func RelativeTime(t time.Time) string {
	return relativeTime(t, time.Now())
}

func relativeTime(t, now time.Time) string {
	d := now.Sub(t)
	if d < time.Minute {
		return "just now"
	}
	return DurationFormat(d) + " ago"
}
