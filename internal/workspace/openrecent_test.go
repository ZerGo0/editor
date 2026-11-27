package workspace

import (
	"path/filepath"
	"slices"
	"testing"

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
	env.OpenFolder = func(_ *mygo.Window, root string, replace bool) { opened = append(opened, openedFolder{root, replace}) }
	env.ForgetFolder = func(root string) { folders = slices.DeleteFunc(folders, func(f string) bool { return f == root }) }
	w := newTestWorkspace(t, folders[0], env, session.Project{})
	writeFiles(t, w.Root, map[string]string{"notes.txt": "notes\n"})
	w.search.recent = []string{filepath.Join(w.Root, "notes.txt")}
	tt := ui.NewTester(w.View, 1200, 800)
	tt.Frame()
	return w, tt, &folders, &opened
}

func TestOpenRecentListsFoldersThenFilesWithoutTheCurrentFolder(t *testing.T) {
	w, _, _, _ := openRecentHarness(t)
	var labels []string
	for _, it := range w.recentEntries() {
		labels = append(labels, it.Group+":"+it.Label)
	}
	if want := []string{"folders:alpha", "folders:beta", "files:notes.txt"}; !slices.Equal(labels, want) {
		t.Fatalf("entries = %v, want %v", labels, want)
	}
}

func TestOpenRecentEnterReplacesAndCmdEnterOpensANewWindow(t *testing.T) {
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
