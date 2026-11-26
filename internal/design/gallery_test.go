package design

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/quickinput"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	shotsEnv     = "DESIGN_SHOTS"
	galleryWidth = 1280
	galleryHigh  = 1500
	quickWidth   = 800
	quickHigh    = 420
)

type fakeCommands map[string]action.CommandInfo

func (f fakeCommands) Lookup(id string) (action.CommandInfo, bool) {
	info, ok := f[id]
	return info, ok
}

func (f fakeCommands) Run(string) {}

func useFakeCommands(t *testing.T) {
	action.SetCommands(fakeCommands{
		galleryBack:    {Title: "Go Back", Keys: "⌃-"},
		galleryForward: {Title: "Go Forward", Keys: "⌃⇧-", Disabled: true},
		galleryOpen:    {Title: "Go to File...", Keys: "⌘P"},
		galleryRefresh: {Title: "Refresh Explorer"},
	})
	t.Cleanup(func() { action.SetCommands(nil) })
}

func saveShot(t *testing.T, tt *ui.Tester, name string) {
	dir := os.Getenv(shotsEnv)
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

func TestGalleryBuilds(t *testing.T) {
	useFakeCommands(t)
	tt := ui.NewTester(func(c *ui.Context) { Gallery(c) }, galleryWidth, galleryHigh)
	tt.Frame()
	for _, name := range []string{"Commit", "Match Case", "Explorer", "Go Back", "Go Forward"} {
		if _, ok := tt.Access(name); !ok {
			t.Errorf("gallery has no element named %q", name)
		}
	}
	saveShot(t, tt, "gallery")
}

func TestQuickInputShot(t *testing.T) {
	items := []quickinput.Item{
		{Label: "gallery.go", Description: "internal/design", Icon: icons.File, Matches: []int{0, 1}, Group: "recently opened"},
		{Label: "palette.go", Description: "internal/design/token", Icon: icons.File},
		{Label: "Go Back", Keys: "⌃-", Group: "commands"},
	}
	for _, p := range []*token.Palette{token.Dark(), token.Light()} {
		s := &quickinput.State{Query: "ga"}
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(p.Theme())
			ui.Column(c).Fill().Background(p.Layer.Editor)
			quickinput.View(c, "Go to File", s, items, quickinput.Options{Placeholder: "Search files by name"})
		}, quickWidth, quickHigh)
		tt.Frame()
		name := "quickinput-light"
		if p.Dark {
			name = "quickinput-dark"
		}
		saveShot(t, tt, name)
	}
}
