package history

import (
	"testing"
	"time"
)

func TestRelativeTime(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		ago  time.Duration
		want string
	}{
		{10 * time.Second, "just now"},
		{time.Minute, "1 minute ago"},
		{5 * time.Minute, "5 minutes ago"},
		{3 * time.Hour, "3 hours ago"},
		{30 * time.Hour, "1 day ago"},
		{3 * 24 * time.Hour, "3 days ago"},
		{15 * 24 * time.Hour, "2 weeks ago"},
	}
	for _, c := range cases {
		if got := relativeTime(now, now.Add(-c.ago)); got != c.want {
			t.Errorf("%v: got %q, want %q", c.ago, got, c.want)
		}
	}
}

func TestFormatSize(t *testing.T) {
	cases := map[int64]string{0: "0 B", 1023: "1023 B", 1536: "1.5 KB", 3 * megabyte: "3.0 MB"}
	for n, want := range cases {
		if got := formatSize(n); got != want {
			t.Errorf("%d: got %q, want %q", n, got, want)
		}
	}
}

func TestAbsoluteTimeShowsYearWhenDifferent(t *testing.T) {
	now := time.Now()
	if got := absoluteTime(now, now.AddDate(-2, 0, 0)); len(got) <= len(absoluteFmt) {
		t.Errorf("no year in %q", got)
	}
}
