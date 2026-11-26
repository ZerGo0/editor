package workspace

import "testing"

func TestWordAt(t *testing.T) {
	tests := []struct {
		line string
		col  int
		want string
	}{
		{"foo := barBaz(1)", 9, "barBaz"},
		{"foo := barBaz(1)", 7, "barBaz"},
		{"foo := barBaz(1)", 13, "barBaz"},
		{"foo := barBaz(1)", 0, "foo"},
		{"foo := barBaz(1)", 5, ""},
		{"snake_case", 99, "snake_case"},
		{"", 0, ""},
	}
	for _, tt := range tests {
		if got := wordAt(tt.line, tt.col); got != tt.want {
			t.Errorf("wordAt(%q, %d) = %q; want %q", tt.line, tt.col, got, tt.want)
		}
	}
}
