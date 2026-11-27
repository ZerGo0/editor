package ghshared_test

import (
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/ghshared"
)

var testNow = time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)

func TestRelativeTime(t *testing.T) {
	const day = 24 * time.Hour
	cases := []struct {
		ago  time.Duration
		want string
	}{
		{10 * time.Second, "just now"},
		{5 * time.Minute, "5m ago"},
		{3 * time.Hour, "3h ago"},
		{3 * day, "3d ago"},
		{15 * day, "2w ago"},
		{364 * day, "51w ago"},
		{400 * day, "1y ago"},
		{800 * day, "2y ago"},
	}
	for _, tc := range cases {
		if got := ghshared.RelativeTime(testNow, testNow.Add(-tc.ago)); got != tc.want {
			t.Errorf("%v: %q, want %q", tc.ago, got, tc.want)
		}
	}
	if ghshared.RelativeTime(testNow, time.Time{}) != "" {
		t.Error("a zero time must give no text")
	}
}
