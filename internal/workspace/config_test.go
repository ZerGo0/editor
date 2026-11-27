package workspace

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/ZerGo0/editor/internal/testutil"
	"github.com/egoist/mygo/ui/editor"
)

func TestZoomLevelScalesLikeVSCode(t *testing.T) {
	const tolerance = 1e-4
	env := newTestEnv(t)
	w := newTestWorkspace(t, t.TempDir(), env, session.Project{})
	for level, want := range map[int]float64{0: 1, 1: 1.2, 2: 1.44, -1: 1 / 1.2, 3: 1.728} {
		if err := env.Settings.Set(settings.ZoomLevel, level); err != nil {
			t.Fatal(err)
		}
		if got := float64(w.zoom()); math.Abs(got-want) > tolerance {
			t.Errorf("zoom at level %d = %v, want %v", level, got, want)
		}
	}
}

func TestDiffSideMatchesEditorConfiguration(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		".editorconfig": "root = true\n[*.go]\nindent_style = tab\ntab_width = 8\n",
		"a.go":          "package a",
	})
	env := newTestEnv(t)
	for key, value := range map[string]any{settings.FontSize: 17, settings.LineHeight: 21, settings.WordWrap: settings.WrapOn} {
		if err := env.Settings.Set(key, value); err != nil {
			t.Fatal(err)
		}
	}
	w := newTestWorkspace(t, root, env, session.Project{})
	path := filepath.Join(root, "a.go")
	tab, _ := openTab(t, w, path)
	w.ConfigureEditor(tab.Editor, path)
	d := &editor.DiffState{}
	w.ConfigureDiff(d, path)
	st := tab.Editor
	if d.Font != st.Font || d.Font.Size != 17 || d.LineHeight != st.LineHeight || d.TabSize != st.TabSize || d.TabSize != 8 || d.Wrap != st.SoftWrap || !d.Wrap {
		t.Fatalf("diff font %+v line %v tab %d wrap %v, editor font %+v line %v tab %d wrap %v",
			d.Font, d.LineHeight, d.TabSize, d.Wrap, st.Font, st.LineHeight, st.TabSize, st.SoftWrap)
	}
}
