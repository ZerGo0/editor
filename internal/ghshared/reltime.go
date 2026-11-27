package ghshared

import (
	"fmt"
	"time"
)

const (
	justNowLimit = 45 * time.Second
	hoursPerDay  = 24
	daysPerWeek  = 7
	weeksPerYear = 52
	daysPerYear  = 365
)

func RelativeTime(now, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := now.Sub(t)
	switch {
	case d < justNowLimit:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d/time.Minute))
	case d < hoursPerDay*time.Hour:
		return fmt.Sprintf("%dh ago", int(d/time.Hour))
	}
	days := int(d / (hoursPerDay * time.Hour))
	switch {
	case days < daysPerWeek:
		return fmt.Sprintf("%dd ago", days)
	case days < daysPerYear:
		weeks := days / daysPerWeek
		if weeks >= weeksPerYear {
			weeks = weeksPerYear - 1
		}
		return fmt.Sprintf("%dw ago", weeks)
	}
	return fmt.Sprintf("%dy ago", days/daysPerYear)
}
