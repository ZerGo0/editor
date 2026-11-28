package history

import (
	"fmt"
	"time"

	"github.com/ZerGo0/editor/internal/plural"
)

const (
	justNow     = 45 * time.Second
	absoluteFmt = "Jan 2, 15:04"
	yearFmt     = "Jan 2 2006, 15:04"
	kilobyte    = 1024
	megabyte    = kilobyte * 1024
	daysPerWeek = 7
)

func relativeTime(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < justNow:
		return "just now"
	case d < time.Hour:
		return ago(int(d/time.Minute), "minute")
	case d < 24*time.Hour:
		return ago(int(d/time.Hour), "hour")
	case d < daysPerWeek*24*time.Hour:
		return ago(int(d/(24*time.Hour)), "day")
	}
	return ago(int(d/(daysPerWeek*24*time.Hour)), "week")
}

func ago(n int, unit string) string {
	return plural.Count(max(n, 1), unit, unit+"s") + " ago"
}

func absoluteTime(now, t time.Time) string {
	t = t.Local()
	if t.Year() != now.Local().Year() {
		return t.Format(yearFmt)
	}
	return t.Format(absoluteFmt)
}

func formatSize(n int64) string {
	switch {
	case n < kilobyte:
		return fmt.Sprintf("%d B", n)
	case n < megabyte:
		return fmt.Sprintf("%.1f KB", float64(n)/kilobyte)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/megabyte)
}
