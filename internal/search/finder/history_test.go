package finder

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestTextQueryHistoryBrowsesWithAltArrowsAndSurvivesRestore(t *testing.T) {
	f := newFixture(t, map[string]string{"a.txt": "alpha beta"})
	for _, q := range []string{"alpha", "beta"} {
		f.dialog.Open(TabText, "")
		f.settle()
		f.key(ui.Cmd, ui.KeyA)
		f.type_(q)
		f.dialog.Close()
		f.settle()
	}
	f.dialog.Open(TabFiles, "")
	f.settle()
	f.dialog.Close()
	if got := f.dialog.Snapshot().History.Entries(); !slices.Equal(got, []string{"alpha", "beta"}) {
		t.Fatalf("history %q, want only the text queries", got)
	}
	data, err := json.Marshal(f.dialog.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatal(err)
	}
	f.dialog.Restore(st)
	f.dialog.Open(TabText, "")
	f.settle()
	f.key(ui.Cmd, ui.KeyA)
	f.type_("draft")
	for i, step := range []struct {
		key  ui.Key
		want string
	}{{ui.KeyUp, "beta"}, {ui.KeyUp, "alpha"}, {ui.KeyDown, "beta"}, {ui.KeyDown, "draft"}} {
		f.key(ui.Alt, step.key)
		f.settle()
		if got := f.dialog.Query(); got != step.want {
			t.Fatalf("step %d: query %q, want %q", i, got, step.want)
		}
		if v, ok := f.tt.AccessByRole(ui.RoleTextField, labelText); !ok || v.Value != step.want {
			t.Fatalf("step %d: field %q (%v), want %q", i, v.Value, ok, step.want)
		}
	}
}
