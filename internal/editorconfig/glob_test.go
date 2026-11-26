package editorconfig

import "testing"

func TestMatch(t *testing.T) {
	tests := []struct {
		glob, path string
		want       bool
	}{
		{"*", "main.go", true},
		{"*", "pkg/main.go", true},
		{"*.go", "pkg/sub/main.go", true},
		{"*.go", "main.js", false},
		{"/*.go", "pkg/main.go", false},
		{"/*.go", "main.go", true},
		{"pkg/*.go", "pkg/main.go", true},
		{"pkg/*.go", "pkg/sub/main.go", false},
		{"pkg/**.go", "pkg/sub/main.go", true},
		{"a/**/z.c", "a/z.c", true},
		{"a/**/z.c", "a/b/c/z.c", true},
		{"**/z.c", "z.c", true},
		{"?.c", "a.c", true},
		{"?.c", "ab.c", false},
		{"[abc].c", "b.c", true},
		{"[abc].c", "d.c", false},
		{"[!abc].c", "d.c", true},
		{"[!abc].c", "a.c", false},
		{"[a-c].c", "b.c", true},
		{"*.{js,ts}", "app.ts", true},
		{"*.{js,ts}", "app.go", false},
		{"{Makefile,*.mk}", "rules.mk", true},
		{"{Makefile,*.mk}", "Makefile", true},
		{"file{1..3}.txt", "file2.txt", true},
		{"file{1..3}.txt", "file4.txt", false},
		{"file{-1..3}.txt", "file-1.txt", true},
		{"{single}.txt", "{single}.txt", true},
		{"\\*.txt", "*.txt", true},
		{"\\*.txt", "a.txt", false},
		{"{a,b{1..2}}.txt", "a.txt", true},
		{"{a,b{1..2}}.txt", "b2.txt", true},
		{"{a,b{1..2}}.txt", "b3.txt", false},
		{"[unclosed.txt", "[unclosed.txt", true},
	}
	for _, tt := range tests {
		if got := Match(tt.glob, tt.path); got != tt.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tt.glob, tt.path, got, tt.want)
		}
	}
}
