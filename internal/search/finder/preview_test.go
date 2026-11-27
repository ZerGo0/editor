package finder

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/design/picker"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const queueWait = 2 * time.Second

func (f *fixture) previewNodes() (group bool, toggles int) {
	f.t.Helper()
	_, group = f.tt.AccessByRole(ui.RoleGroup, previewLabel)
	for _, a := range f.tt.AccessAll() {
		if a.Name == previewLabel && a.Role != ui.RoleGroup {
			toggles++
		}
	}
	return group, toggles
}

func (f *fixture) waitQueue() {
	f.t.Helper()
	deadline := time.Now().Add(queueWait)
	for len(f.host.queue) == 0 {
		if time.Now().After(deadline) {
			f.t.Fatal("the preview did not finish loading")
		}
		time.Sleep(settleWait)
	}
}

func TestPreviewShowsOnlyOnFilesAndTextTabs(t *testing.T) {
	for _, tc := range []struct {
		tab  Tab
		seed string
		want bool
	}{{TabAll, "", false}, {TabActions, "", false}, {TabFiles, "", true}, {TabText, "needle", true}} {
		f := newFixture(t, demoProject)
		f.dialog.st.Preview = true
		f.open(tc.tab, tc.seed)
		group, toggles := f.previewNodes()
		if group != tc.want || (toggles > 0) != tc.want {
			t.Errorf("%v: preview pane %v, toggle buttons %d; want shown %v", tc.tab, group, toggles, tc.want)
		}
		if f.dialog.Snapshot().Preview != true {
			t.Errorf("%v: the preview setting was lost", tc.tab)
		}
	}
}

func TestPreviewSettingSurvivesTabsWithoutPreview(t *testing.T) {
	f := newFixture(t, demoProject)
	f.dialog.st.Preview = true
	f.open(TabFiles, "")
	f.click("Actions")
	if group, toggles := f.previewNodes(); group || toggles != 0 {
		t.Fatal("the Actions tab shows the preview or its toggle")
	}
	f.click("Files")
	if group, toggles := f.previewNodes(); !group || toggles == 0 {
		t.Fatal("the Files tab does not show the preview again")
	}
}

func TestPreviewKeepsTheOldFileWhileTheNextOneLoads(t *testing.T) {
	f := newFixture(t, demoProject)
	now := time.Unix(1, 0)
	f.dialog.preview.now = func() time.Time { return now }
	f.dialog.st.Preview = true
	f.recents = []string{f.path("main.go"), f.path("docs/readme.md")}
	f.open(TabFiles, "")
	if f.dialog.preview.st == nil || !strings.Contains(f.dialog.preview.st.Buffer.String(), "package main") {
		t.Fatal("the first file is not previewed")
	}
	first := f.dialog.preview.st
	f.tt.Key(0, ui.KeyDown)
	loadingFrames := 0
	for range 3 {
		f.tt.Frame()
		if f.tt.HasText(msgLoading) {
			loadingFrames++
		}
		if f.dialog.preview.st != first {
			t.Fatal("the old preview was dropped before the new one was ready")
		}
	}
	f.waitQueue()
	f.settle()
	if loadingFrames != 0 {
		t.Fatalf("%d frames showed the loading state for a fast selection change; want 0", loadingFrames)
	}
	if f.dialog.preview.st == first || !strings.Contains(f.dialog.preview.st.Buffer.String(), "# Title") {
		t.Fatal("the preview did not move to the new file")
	}
	if f.tt.HasText(msgLoading) {
		t.Fatal("the loading state stays after the load")
	}
}

func TestPreviewShowsLoadingOnlyAfterTheSpinnerDelay(t *testing.T) {
	f := newFixture(t, demoProject)
	now := time.Unix(1, 0)
	f.dialog.preview.now = func() time.Time { return now }
	f.dialog.st.Preview = true
	f.recents = []string{f.path("main.go"), f.path("docs/readme.md")}
	f.open(TabFiles, "")
	f.tt.Key(0, ui.KeyDown)
	now = now.Add(token.SpinnerDelay - time.Millisecond)
	f.tt.Frame()
	if f.tt.HasText(msgLoading) {
		t.Fatal("the loading state shows before the spinner delay")
	}
	now = now.Add(time.Millisecond)
	f.tt.Frame()
	if !f.tt.HasText(msgLoading) {
		t.Fatal("the loading state does not show for a slow load")
	}
	f.waitQueue()
	f.settle()
	if f.tt.HasText(msgLoading) || f.dialog.preview.st == nil {
		t.Fatal("the preview does not replace the loading state")
	}
}

func TestPreviewDoesNotRebuildForTheSameFile(t *testing.T) {
	f := newFixture(t, map[string]string{"a.txt": "needle one\nneedle two\n", "b.txt": "other\n"})
	f.dialog.st.Preview = true
	f.open(TabText, "needle")
	st := f.dialog.preview.st
	if st == nil {
		t.Fatal("no preview of the first match")
	}
	before := st.CursorPos()
	f.key(0, ui.KeyDown)
	if f.dialog.preview.st != st {
		t.Fatal("the preview was rebuilt for a match in the same file")
	}
	if after := st.CursorPos(); after.Line <= before.Line {
		t.Fatalf("the preview cursor stayed on line %d, want it on the next match", after.Line)
	}
	if f.dialog.preview.pending {
		t.Fatal("moving inside one file starts a load")
	}
}

func TestPreviewTitleShowsTheRelativePath(t *testing.T) {
	f := newFixture(t, demoProject)
	f.dialog.st.Preview = true
	f.open(TabText, "needle")
	if f.dialog.preview.title != "docs/readme.md" && f.dialog.preview.title != "server/server.go" {
		t.Fatalf("preview title %q is not a relative path", f.dialog.preview.title)
	}
	if !f.tt.HasText(f.dialog.preview.title) {
		t.Fatal("the preview header is not drawn")
	}
}

func TestDialogLayoutIsSavedAndRestored(t *testing.T) {
	f := newFixture(t, demoProject)
	f.dialog.Restore(State{Tab: "Files", Layout: picker.Layout{Width: 700, Height: 400, Preview: 150}})
	f.open(TabFiles, "")
	a, ok := f.tt.AccessByRole(ui.RoleDialog, labelFiles)
	if !ok || a.Bounds.W != 700 || a.Bounds.H != 400 {
		t.Fatalf("dialog %+v, want the saved 700x400", a.Bounds)
	}
	data, err := json.Marshal(f.dialog.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	var again State
	if err := json.Unmarshal(data, &again); err != nil || again.Layout != (picker.Layout{Width: 700, Height: 400, Preview: 150}) {
		t.Fatalf("snapshot %s restores %+v (%v)", data, again.Layout, err)
	}
}

func TestDraggingTheDialogEdgeSavesTheSize(t *testing.T) {
	f := newFixture(t, demoProject)
	f.dialog.Restore(State{Tab: "Files", Layout: picker.Layout{Width: 700, Height: 400}})
	f.open(TabFiles, "")
	edge, ok := f.tt.AccessByRole(ui.RoleSplitter, "Resize Dialog Right")
	if !ok {
		t.Fatal("no right edge handle")
	}
	x, y := edge.Bounds.X+edge.Bounds.W/2, edge.Bounds.Y+edge.Bounds.H/2
	f.tt.Press(x, y)
	f.tt.Move(x+20, y)
	f.tt.Frame()
	f.tt.Move(x+40, y)
	f.tt.Frame()
	f.tt.Release(x+40, y)
	f.settle()
	if got := f.dialog.Snapshot().Layout.Width; got != 780 {
		t.Fatalf("saved width %d, want 780", got)
	}
	if !f.dialog.IsOpen() {
		t.Fatal("dragging the edge closed the dialog")
	}
}
