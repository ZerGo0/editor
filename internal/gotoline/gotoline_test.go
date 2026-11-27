package gotoline

import "testing"

func lengths(n int) func(int) int { return func(int) int { return n } }

func TestParse(t *testing.T) {
	const lines, width = 10, 5
	for _, tt := range []struct {
		input string
		want  Target
	}{
		{"3", Target{Line: 3, LineOK: true}},
		{" 3 ", Target{Line: 3, LineOK: true}},
		{"3:2", Target{Line: 3, Column: 2, LineOK: true, ColumnOK: true}},
		{"3,2", Target{Line: 3, Column: 2, LineOK: true, ColumnOK: true}},
		{"3#2", Target{Line: 3, Column: 2, LineOK: true, ColumnOK: true}},
		{":3", Target{Line: 3, LineOK: true}},
		{"3:", Target{Line: 3, LineOK: true}},
		{"3:6", Target{Line: 3, Column: 6, LineOK: true, ColumnOK: true}},
		{"3:7", Target{Line: 3, Column: 7, LineOK: true}},
		{"3:0", Target{Line: 3, LineOK: true}},
		{"3:-1", Target{Line: 3, Column: -1, LineOK: true}},
		{"10", Target{Line: 10, LineOK: true}},
		{"11", Target{Line: 11}},
		{"0", Target{Line: 11}},
		{"-1", Target{Line: 10, LineOK: true}},
		{"-10", Target{Line: 1, LineOK: true}},
		{"-10:2", Target{Line: 1, Column: 2, LineOK: true, ColumnOK: true}},
		{"-11", Target{Line: 0}},
		{"abc", Target{}},
		{"", Target{}},
		{"abc:4", Target{Line: 4, LineOK: true}},
		{"4abc", Target{Line: 4, LineOK: true}},
	} {
		if got := Parse(tt.input, lines, lengths(width)); got != tt.want {
			t.Errorf("Parse(%q) = %+v, want %+v", tt.input, got, tt.want)
		}
	}
}

func TestLabels(t *testing.T) {
	invalid := Target{}
	for _, tt := range []struct {
		target Target
		count  int
		want   string
	}{
		{Target{Line: 3, LineOK: true}, 10, "Go to line 3."},
		{Target{Line: 3, Column: 2, LineOK: true, ColumnOK: true}, 10, "Go to line 3 and character 2."},
		{invalid, 10, "Current Line: 4, Character: 2. Type a line number between 1 and 10 to navigate to."},
		{invalid, 1, "Current Line: 4, Character: 2. Type a line number to navigate to."},
	} {
		if got := tt.target.Label(4, 2, tt.count); got != tt.want {
			t.Errorf("Label = %q, want %q", got, tt.want)
		}
	}
}

func TestByteColumnCountsCharacters(t *testing.T) {
	const text = "aé中z"
	for column, want := range map[int]int{1: 0, 2: 1, 3: 3, 4: 6, 5: 7, 9: 7} {
		if got := ByteColumn(text, column); got != want {
			t.Errorf("ByteColumn(%d) = %d, want %d", column, got, want)
		}
	}
}
