package plural

import "testing"

func TestCount(t *testing.T) {
	for _, c := range []struct {
		n    int
		want string
	}{{0, "0 matches"}, {1, "1 match"}, {2, "2 matches"}} {
		if got := Count(c.n, "match", "matches"); got != c.want {
			t.Errorf("Count(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
