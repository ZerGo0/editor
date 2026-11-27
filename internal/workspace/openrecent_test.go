package workspace

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/backup"

	"github.com/ZerGo0/editor/internal/session"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type openedFolder struct {
	root    string
	replace bool
}

func openRecentHarness(t *testing.T) (*Workspace, *ui.Tester, *[]string, *[]openedFolder) {
	t.Helper()
	env := newTestEnv(t)
	parent := t.TempDir()
	folders := []string{filepath.Join(parent, "current"), filepath.Join(parent, "alpha"), filepath.Join(parent, "beta")}
	var opened []openedFolder
	env.RecentFolders = func() []string { return folders }
	open := []string{folders[0], folders[2]}
	env.OpenFolders = func() []string { return open }
	env.OpenFolder = func(_ *mygo.Window, root string, replace bool) { opened = append(opened, openedFolder{root, replace}) }
	env.ForgetFolder = func(root string) { folders = slices.DeleteFunc(folders, func(f string) bool { return f == root }) }
	w := newTestWorkspace(t, folders[0], env, session.Project{})
	writeFiles(t, w.Root, map[string]string{"notes.txt": "notes\n"})
	w.search.recent = []string{filepath.Join(w.Root, "notes.txt")}
	tt := ui.NewTester(w.View, 1200, 800)
	tt.Frame()
	return w, tt, &folders, &opened
}

func TestOpenRecentListsTheCurrentFolderLikeVSCode(t *testing.T) {
	w, _, _, _ := openRecentHarness(t)
	var labels []string
	for _, it := range w.recentEntries() {
		labels = append(labels, it.Group+":"+it.Label+":"+it.Action.Label)
	}
	want := []string{
		"folders:current:" + openedHereLabel,
		"folders:alpha:Remove alpha from Recently Opened",
		"folders:beta:" + openedElsewhereLabel,
		"files:notes.txt:Remove notes.txt from Recently Opened",
	}
	if !slices.Equal(labels, want) {
		t.Fatalf("entries = %v, want %v", labels, want)
	}
}

func TestOpenRecentStartsOnTheEntryAfterTheCurrentFolder(t *testing.T) {
	w, tt, _, opened := openRecentHarness(t)
	alpha := filepath.Join(filepath.Dir(w.Root), "alpha")
	w.ShowOpenRecent()
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	w.ShowOpenRecent()
	tt.Frame()
	tt.Key(ui.Super, ui.KeyEnter)
	if want := []openedFolder{{alpha, true}, {alpha, false}}; !slices.Equal(*opened, want) {
		t.Fatalf("opened %v, want %v", *opened, want)
	}
	if w.palette.IsOpen() {
		t.Error("the palette stays open after opening a folder")
	}
}

func TestOpenRecentOpensAFileInThisWindow(t *testing.T) {
	w, tt, _, opened := openRecentHarness(t)
	w.ShowOpenRecent()
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	if tab := w.active.ActiveTab(); tab == nil || filepath.Base(tab.Path) != "notes.txt" || len(*opened) != 0 {
		t.Fatalf("active tab %v, folders opened %v; want notes.txt and none", tab, *opened)
	}
}

func TestOpenRecentRemoveButtonForgetsTheEntry(t *testing.T) {
	w, tt, folders, _ := openRecentHarness(t)
	w.ShowOpenRecent()
	tt.Frame()
	if err := tt.Click("Remove alpha from Recently Opened"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if slices.ContainsFunc(*folders, func(f string) bool { return filepath.Base(f) == "alpha" }) || !w.palette.IsOpen() {
		t.Fatalf("folders %v, palette open %v; want alpha gone and the palette open", *folders, w.palette.IsOpen())
	}
	if tt.HasText("alpha") || !tt.HasText("beta") {
		t.Error("the list still shows alpha or lost beta")
	}
	tt.Key(0, ui.KeyDown)
	if err := tt.Click("Remove notes.txt from Recently Opened"); err != nil || len(w.search.recent) != 0 {
		t.Fatalf("remove file: %v, recent %v", err, w.search.recent)
	}
}

func TestOpenRecentShowsRecentFilesFromEveryProject(t *testing.T) {
	env := newTestEnv(t)
	root := t.TempDir()
	other := filepath.Join(t.TempDir(), "elsewhere.go")
	writeFiles(t, root, map[string]string{"mine.go": "package a\n"})
	global := []string{other, filepath.Join(root, "mine.go")}
	var touched, forgot []string
	env.RecentFiles = func() []string { return global }
	env.TouchFile = func(p string) { touched = append(touched, p) }
	env.ForgetFile = func(p string) { forgot = append(forgot, p) }
	w := newTestWorkspace(t, root, env, session.Project{})
	if !slices.Equal(w.search.recent, []string{filepath.Join(root, "mine.go")}) {
		t.Fatalf("project recent files = %v, want only the file inside the project", w.search.recent)
	}
	var files []string
	for _, it := range w.recentEntries() {
		if it.Group == recentFilesGroup {
			files = append(files, it.Label)
		}
	}
	if !slices.Equal(files, []string{"elsewhere.go", "mine.go"}) {
		t.Fatalf("Open Recent files = %v, want both projects", files)
	}
	w.OpenFile(filepath.Join(root, "mine.go"), false, true)
	w.forgetRecent(other)
	if !slices.Contains(touched, filepath.Join(root, "mine.go")) || !slices.Equal(forgot, []string{other}) {
		t.Fatalf("touched %v, forgot %v", touched, forgot)
	}
}

func TestOpenRecentMarksFoldersWithUnsavedFiles(t *testing.T) {
	w, _, folders, _ := openRecentHarness(t)
	w.env.BackupDir = t.TempDir()
	alpha := (*folders)[1]
	store := backup.Open(filepath.Join(w.env.BackupDir, projectKey(alpha)), backup.DefaultLimit)
	body := "unsaved"
	if err := store.Write(1, backup.Meta{Path: filepath.Join(alpha, "a.go"), Size: int64(len(body))}, strings.NewReader(body), int64(len(body))); err != nil {
		t.Fatal(err)
	}
	_, e := openTab(t, w, filepath.Join(w.Root, "notes.txt"))
	e.doc.Buffer.Set("edited\n")
	labels := map[string]string{}
	for _, it := range w.recentEntries() {
		labels[it.Label] = it.Action.Label
	}
	if labels["current"] != unsavedFolderLabel || labels["alpha"] != unsavedFolderLabel || labels["beta"] != openedElsewhereLabel {
		t.Fatalf("buttons = %v", labels)
	}
}
