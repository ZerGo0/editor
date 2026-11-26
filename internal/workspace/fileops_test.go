package workspace

import "testing"

func TestFreeCopyName(t *testing.T) {
	taken := map[string]bool{"a.txt": true, "a copy.txt": true, ".env": true, "dir": true, "dir copy": true, "dir copy 2": true}
	exists := func(name string) bool { return taken[name] }
	for in, want := range map[string]string{"a.txt": "a copy 2.txt", ".env": ".env copy", "dir": "dir copy 3", "b.go": "b.go"} {
		got, err := freeCopyName("", in, exists)
		if err != nil || got != want {
			t.Errorf("freeCopyName(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}
