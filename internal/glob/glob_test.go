package glob

import "testing"

func TestMatch(t *testing.T) {
	tests := []struct {
		pattern, rel string
		want         bool
	}{
		{"**/.git", ".git", true},
		{"**/.git", "sub/.git", true},
		{"**/.git", "sub/.gitignore", false},
		{"**/node_modules", "a/b/node_modules", true},
		{"*.log", "logs/app.log", true},
		{"*.log", "app.txt", false},
		{"build", "build", true},
		{"build", "src/build", true},
		{"/build/", "build", true},
		{"src/*.go", "src/main.go", true},
		{"src/*.go", "src/sub/main.go", false},
		{"src/**/*.go", "src/main.go", true},
		{"src/**/*.go", "src/a/b/main.go", true},
		{"src/**", "src/a/b", true},
		{"src/**", "other/a", false},
	}
	for _, tt := range tests {
		if got := Match(tt.pattern, tt.rel); got != tt.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tt.pattern, tt.rel, got, tt.want)
		}
	}
}

func TestMatchAny(t *testing.T) {
	if !MatchAny([]string{"*.tmp", "**/.git"}, "x/.git") {
		t.Error("want match")
	}
	if MatchAny(nil, "x") {
		t.Error("want no match")
	}
}
