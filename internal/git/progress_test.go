package git

import "testing"

func TestParseProgress(t *testing.T) {
	t.Parallel()
	cases := []struct {
		line  string
		phase string
		done  float64
		ok    bool
	}{
		{"Receiving objects:  50% (5/10), 1.00 KiB | 1.00 MiB/s", "Receiving objects 50%", 0.45, true},
		{"remote: Counting objects: 100% (2/2), done.", "Counting objects 100%", 0.1, true},
		{"Resolving deltas:   0% (0/3)", "Resolving deltas 0%", 0.7, true},
		{"Unpacking objects:  40% (2/5)", "Unpacking objects 40%", 0.4, true},
		{"From /tmp/remote", "", 0, false},
		{"Receiving objects: 300%", "", 0, false},
	}
	for _, tc := range cases {
		phase, done, ok := ParseProgress(tc.line)
		if phase != tc.phase || ok != tc.ok || (ok && (done < tc.done-1e-9 || done > tc.done+1e-9)) {
			t.Errorf("ParseProgress(%q) = %q, %v, %v", tc.line, phase, done, ok)
		}
	}
}
