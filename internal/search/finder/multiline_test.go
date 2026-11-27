package finder

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

const wrapWords = 60

func TestTextQueryTakesSeveralLines(t *testing.T) {
	f := newFixture(t, map[string]string{"a.txt": "x alpha\nbeta y\n", "b.txt": "alpha\n"})
	f.dialog.Open(TabText, "")
	f.settle()
	f.type_("alpha")
	f.key(ui.Ctrl, ui.KeyEnter)
	f.type_("beta")
	if got := f.dialog.st.Query; got != "alpha\nbeta" {
		t.Fatalf("Ctrl+Enter left the query at %q, want a line break", got)
	}
	if labels := strings.Join(f.labels(), "|"); !strings.Contains(labels, "x alpha") || strings.Count(labels, "line ") != 1 {
		t.Fatalf("the two-line query found %q, want only the match in a.txt", labels)
	}
}

func TestTextQueryArrowsMoveTheCaretUntilTheEdgeLikeVSCode(t *testing.T) {
	long := strings.TrimSpace(strings.Repeat("word ", wrapWords))
	for name, query := range map[string]string{"two lines": "alpha\nbeta", "wrapped line": long} {
		f := newFixture(t, map[string]string{"a.txt": query + "\n", "b.txt": query + "\n"})
		f.dialog.Open(TabText, query)
		f.settle()
		start := f.dialog.results.Selected
		f.key(ui.Cmd, ui.KeyUp)
		f.key(0, ui.KeyDown)
		if f.dialog.results.Selected != start {
			t.Fatalf("%s: Down with the caret at the start moved the results from %d to %d", name, start, f.dialog.results.Selected)
		}
		f.key(ui.Cmd, ui.KeyDown)
		f.key(0, ui.KeyUp)
		if f.dialog.results.Selected != start {
			t.Fatalf("%s: Up with the caret at the end moved the results from %d to %d", name, start, f.dialog.results.Selected)
		}
		f.key(ui.Cmd, ui.KeyDown)
		f.key(0, ui.KeyDown)
		if f.dialog.results.Selected == start {
			t.Fatalf("%s: Down with the caret at the end did not move the results from %d", name, start)
		}
	}
}

func TestShortTextQueryArrowsMoveTheResults(t *testing.T) {
	f := newFixture(t, map[string]string{"a.txt": "alpha\n", "b.txt": "alpha\n"})
	f.dialog.Open(TabText, "alpha")
	f.settle()
	start := f.dialog.results.Selected
	f.key(ui.Cmd, ui.KeyUp)
	f.key(0, ui.KeyDown)
	if f.dialog.results.Selected == start {
		t.Fatalf("Down in a one-line query did not move the results from %d", start)
	}
}

func TestTextToggleKeysSwitchTheOptions(t *testing.T) {
	f := newFixture(t, map[string]string{"a.txt": "Alpha\nalpha\n"})
	f.dialog.Open(TabText, "")
	f.settle()
	f.type_("alpha")
	before := len(f.labels())
	f.key(ui.Alt|ui.Cmd, ui.KeyC)
	if !f.dialog.st.MatchCase || len(f.labels()) >= before {
		t.Fatalf("Alt+Cmd+C: match case %v, %d results from %d", f.dialog.st.MatchCase, len(f.labels()), before)
	}
	f.key(ui.Alt|ui.Cmd, ui.KeyW)
	f.key(ui.Alt|ui.Cmd, ui.KeyR)
	if !f.dialog.st.WholeWord || !f.dialog.st.Regex {
		t.Fatalf("Alt+Cmd+W and R left whole word %v and regex %v", f.dialog.st.WholeWord, f.dialog.st.Regex)
	}
	f.key(ui.Alt|ui.Cmd, ui.KeyP)
	if f.dialog.st.PreserveCase {
		t.Fatal("Alt+Cmd+P turned on Preserve Case while the replace field is hidden")
	}
}
