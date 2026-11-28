package workspace

import (
	"os"
	"testing"

	"github.com/ZerGo0/editor/internal/problems"
	"github.com/ZerGo0/editor/internal/session"
)

const (
	brokenSettings = "{\n  \"editor.tabSize\": \"x\"\n}"
	validSettings  = "{\n  \"editor.tabSize\": 2\n}"
)

func countNotices(w *Workspace, title string) int {
	n := 0
	for _, note := range w.notes.List() {
		if note.Title == title {
			n++
		}
	}
	return n
}

func TestBrokenSettingsShowOneNoticePerErrorSet(t *testing.T) {
	env := newTestEnv(t)
	if err := os.WriteFile(env.Settings.Path, []byte(brokenSettings), 0o644); err != nil {
		t.Fatal(err)
	}
	env.Settings.Reload()
	w := newTestWorkspace(t, t.TempDir(), env, session.Project{})
	const title = "settings.json has errors"
	for range 3 {
		w.updateProblems()
	}
	if n := countNotices(w, title); n != 1 {
		t.Fatalf("%d notices after three frames, want 1", n)
	}
	for _, note := range w.notes.List() {
		if note.Title == title && (len(note.Actions) != 1 || note.Actions[0].Label != openSettingsJSON || note.Detail != settingsErrorDetail) {
			t.Fatalf("notice = %+v", note)
		}
	}
	if err := os.WriteFile(env.Settings.Path, []byte(validSettings), 0o644); err != nil {
		t.Fatal(err)
	}
	env.Settings.Reload()
	w.updateProblems()
	if len(w.insights.configShown) != 0 {
		t.Fatalf("shown = %v after the file was fixed", w.insights.configShown)
	}
	if err := os.WriteFile(env.Settings.Path, []byte("{\n  \"editor.nope\": 1\n}"), 0o644); err != nil {
		t.Fatal(err)
	}
	env.Settings.Reload()
	w.updateProblems()
	if w.insights.configShown[env.Settings.Path] == "" {
		t.Fatal("a new error after a fix is not tracked for a new notice")
	}
}

func TestProblemBadgeCountsEntries(t *testing.T) {
	env := newTestEnv(t)
	text := "{\n  \"editor.tabSize\": \"x\",\n  \"editor.nope\": 1\n}"
	if err := os.WriteFile(env.Settings.Path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	env.Settings.Reload()
	w := newTestWorkspace(t, t.TempDir(), env, session.Project{})
	if n := w.problemCount(); n != 2 {
		t.Fatalf("badge = %d, want 2 entries", n)
	}
	if got := w.problemCounts()[problems.Error]; got != 2 {
		t.Fatalf("status count = %d, want 2", got)
	}
}
