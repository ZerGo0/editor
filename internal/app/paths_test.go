package app

import (
	"slices"
	"testing"
)

func TestReplaceWindowOrder(t *testing.T) {
	for name, tc := range map[string]struct {
		confirmed, opened bool
		want              []string
	}{
		"cancel keeps the window and opens nothing": {confirmed: false, opened: true, want: nil},
		"confirm opens the folder, then closes":     {confirmed: true, opened: true, want: []string{"open", "close"}},
		"a folder that does not open keeps window":  {confirmed: true, opened: false, want: []string{"open"}},
	} {
		t.Run(name, func(t *testing.T) {
			var calls []string
			replaceWindow(
				func(done func(ok bool)) { done(tc.confirmed) },
				func() bool { calls = append(calls, "open"); return tc.opened },
				func() { calls = append(calls, "close") },
			)
			if !slices.Equal(calls, tc.want) {
				t.Fatalf("calls = %v, want %v", calls, tc.want)
			}
		})
	}
}
