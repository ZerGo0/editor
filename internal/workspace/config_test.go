package workspace

import (
	"math"
	"testing"

	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/settings"
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
