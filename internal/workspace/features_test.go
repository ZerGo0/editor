package workspace

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/document"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	testFilePerm = 0o644
	testViewW    = 400
	testViewH    = 600
	testWideW    = 1000
)

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, data := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), dirPerm); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), testFilePerm); err != nil {
			t.Fatal(err)
		}
	}
}

func newTestEnv(t *testing.T) *Env {
	t.Helper()
	userData := t.TempDir()
	return &Env{
		Settings:  settings.Load(filepath.Join(userData, "settings.json")),
		Keys:      keymap.Load(filepath.Join(userData, "keybindings.json")),
		Commands:  command.NewRegistry(),
		UserData:  userData,
		BackupDir: filepath.Join(userData, "backups"),
	}
}

func newTestWorkspace(t *testing.T, root string, env *Env, saved session.Project) *Workspace {
	t.Helper()
	w := New(root, saved, env)
	t.Cleanup(w.Close)
	return w
}

func openTab(t *testing.T, w *Workspace, p string) (*layout.Tab, *docEntry) {
	t.Helper()
	w.OpenFile(p, false, true)
	tab := w.active.ActiveTab()
	e := w.ensureEditor(tab)
	if e.err != nil || e.doc == nil {
		t.Fatalf("open %s: %v", p, e.err)
	}
	return tab, e
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestNewUntitledNumbersTabs(t *testing.T) {
	w := newTestWorkspace(t, t.TempDir(), newTestEnv(t), session.Project{})
	w.NewUntitled()
	w.NewUntitled()
	var titles []string
	for _, tab := range w.tabs() {
		titles = append(titles, w.tabTitle(tab.Path))
		if e := w.entry(tab.Path); e.doc == nil || !e.doc.Editable() || e.doc.Modified() {
			t.Fatalf("%s: untitled document is not an empty editable text", tab.Path)
		}
	}
	if strings.Join(titles, ",") != "Untitled-1,Untitled-2" {
		t.Fatalf("titles = %v", titles)
	}
	if w.restorable(w.tabs()[0].Path) {
		t.Fatal("an untitled tab without a backup is restorable")
	}
	if got := w.abs(w.tabs()[0].Path); got != w.tabs()[0].Path {
		t.Fatalf("abs changed an untitled path to %s", got)
	}
}

func TestCmdNOpensUntitledFile(t *testing.T) {
	env := newTestEnv(t)
	b, ok := env.Keys.Current.Lookup(keymap.Chord{{Mods: ui.Cmd, Key: ui.KeyN}})
	if !ok || b.Command != "workbench.action.files.newUntitledFile" {
		t.Fatalf("Cmd+N runs %q", b.Command)
	}
}

func TestFinishSaveAsTurnsUntitledIntoFile(t *testing.T) {
	root := t.TempDir()
	w := newTestWorkspace(t, root, newTestEnv(t), session.Project{})
	w.NewUntitled()
	tab := w.active.ActiveTab()
	d := w.entry(tab.Path).doc
	d.Buffer.Set("hello")
	target := filepath.Join(root, "hello.txt")
	if !w.finishSaveAs(d, target) {
		t.Fatal("save as failed")
	}
	if tab.Path != target || d.Modified() || readFile(t, target) != "hello" {
		t.Fatalf("tab %s modified %v", tab.Path, d.Modified())
	}
}

func TestHotExitRestoresUnsavedText(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"a.txt": "disk\n"})
	env := newTestEnv(t)
	w := newTestWorkspace(t, root, env, session.Project{})
	p := filepath.Join(root, "a.txt")
	_, e := openTab(t, w, p)
	e.doc.Buffer.Set("edited\n")
	w.NewUntitled()
	untitled := w.active.ActiveTab().Path
	w.entry(untitled).doc.Buffer.Set("draft text")
	if w.PrepareClose() {
		t.Fatal("hot exit asks to save backed-up documents")
	}
	snap := w.Snapshot()
	writeFiles(t, root, map[string]string{"a.txt": "changed on disk\n"})

	w2 := newTestWorkspace(t, root, env, snap)
	for path, want := range map[string]string{p: "edited\n", untitled: "draft text"} {
		d := w2.entry(path).doc
		if d == nil || d.Buffer.String() != want || !d.Modified() {
			t.Fatalf("%s did not restore %q", path, want)
		}
	}
	if notes := w2.notes.List(); len(notes) == 0 || !strings.Contains(notes[0].Title, "changed on disk") {
		t.Fatalf("notices = %v", notes)
	}
	if !w2.save([]*document.Document{w2.doc(p)}) {
		t.Fatal("save failed")
	}
	w2.syncBackups()
	if w2.hasBackup(p) {
		t.Fatal("the backup stays after save")
	}
	if !w2.hasBackup(untitled) {
		t.Fatal("the untitled backup is gone")
	}
}

func TestHotExitOffAsksToSave(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"a.txt": "disk"})
	env := newTestEnv(t)
	if err := env.Settings.Set(settings.HotExit, settings.HotExitOff); err != nil {
		t.Fatal(err)
	}
	w := newTestWorkspace(t, root, env, session.Project{})
	_, e := openTab(t, w, filepath.Join(root, "a.txt"))
	e.doc.Buffer.Set("edited")
	if !w.PrepareClose() || len(w.hot.store.Entries()) != 0 {
		t.Fatal("hot exit off kept a backup or skipped the prompt")
	}
}

func TestEditorConfigOverridesSettings(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		".editorconfig": "root = true\n[*.go]\nindent_style = tab\ntab_width = 8\nmax_line_length = 100\ninsert_final_newline = true\nend_of_line = crlf\n",
		"a.go":          "package a",
		"b.txt":         "text",
	})
	w := newTestWorkspace(t, root, newTestEnv(t), session.Project{})
	tab, e := openTab(t, w, filepath.Join(root, "a.go"))
	w.ConfigureEditor(tab.Editor, tab.Path)
	st := tab.Editor
	if st.Indent != editor.IndentTabs || st.TabSize != 8 || !slices.Equal(st.WrapGuides, []int{100}) {
		t.Fatalf("indent %v size %d guides %v", st.Indent, st.TabSize, st.WrapGuides)
	}
	e.doc.Buffer.Set("package b")
	if !w.save([]*document.Document{e.doc}) {
		t.Fatal("save failed")
	}
	if got := readFile(t, filepath.Join(root, "a.go")); got != "package b\r\n" {
		t.Fatalf("saved %q", got)
	}
	other, _ := openTab(t, w, filepath.Join(root, "b.txt"))
	w.ConfigureEditor(other.Editor, other.Path)
	if other.Editor.Indent == editor.IndentTabs {
		t.Fatal("the [*.go] section applied to b.txt")
	}
}

func TestFolderSettingsOverrideUserSettings(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		".editor/settings.json": `{"files.exclude": ["*.log"], "window.zoomLevel": 3}`,
	})
	env := newTestEnv(t)
	if err := env.Settings.Set(settings.Exclude, []string{"*.tmp"}); err != nil {
		t.Fatal(err)
	}
	w := newTestWorkspace(t, root, env, session.Project{})
	if !w.excluded(filepath.Join(root, "x.log")) || w.excluded(filepath.Join(root, "x.tmp")) {
		t.Fatal("folder files.exclude does not override the user value")
	}
	if w.settings.Int(settings.ZoomLevel) != 0 || len(w.settings.Errors) == 0 {
		t.Fatal("folder settings accepted an app-wide key")
	}
}

func TestBracketColorsFollowSetting(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"a.go": "f(a[b{c}])"})
	env := newTestEnv(t)
	w := newTestWorkspace(t, root, env, session.Project{})
	tab, _ := openTab(t, w, filepath.Join(root, "a.go"))
	tt := ui.NewTester(func(c *ui.Context) {
		editor.View(c, tab.Editor).Fill()
		w.colorBrackets(c, tab)
	}, testViewW, testViewH)
	tt.Frame()
	tt.Frame()
	rs := tab.Editor.Replacements()
	if len(rs) != 6 || rs[0].Color != rs[5].Color || rs[0].Color == rs[1].Color {
		t.Fatalf("replacements %+v", rs)
	}
	if err := env.Settings.Set(settings.BracketColors, false); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(tab.Editor.Replacements()) != 0 {
		t.Fatal("bracket colors stay after the setting turns off")
	}
}
