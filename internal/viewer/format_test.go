package viewer

import (
	"testing"
	"time"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{{0, "0 bytes"}, {1023, "1023 bytes"}, {1024, "1.0 KB"}, {1536, "1.5 KB"}, {5 << 20, "5.0 MB"}, {3 << 30, "3.0 GB"}, {2 << 40, "2.0 TB"}, {5000 << 40, "5000.0 TB"}}
	for _, tt := range tests {
		if got := formatSize(tt.n); got != tt.want {
			t.Errorf("formatSize(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestFormatCount(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{{0, "0"}, {999, "999"}, {1000, "1,000"}, {1234567, "1,234,567"}, {1 << 30, "1,073,741,824"}}
	for _, tt := range tests {
		if got := formatCount(tt.n); got != tt.want {
			t.Errorf("formatCount(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestFormatClock(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{{0, "0:00"}, {-time.Second, "0:00"}, {65 * time.Second, "1:05"}, {time.Hour + 2*time.Minute + 3*time.Second, "1:02:03"}, {999 * time.Millisecond, "0:00"}}
	for _, tt := range tests {
		if got := formatClock(tt.d); got != tt.want {
			t.Errorf("formatClock(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestJoinInfo(t *testing.T) {
	if got := joinInfo("a", "", "b"); got != "a"+infoSeparator+"b" {
		t.Errorf("joinInfo = %q", got)
	}
	if joinInfo("", "") != "" {
		t.Error("joinInfo of nothing is not empty")
	}
}
