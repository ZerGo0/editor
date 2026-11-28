package reviewui

import (
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/review"
	"github.com/egoist/mygo/ui"
)

const (
	sidebarTitle       = "Annotations"
	globalTitle        = "Global comments"
	addGlobalLabel     = "Add Global Comment"
	globalComposer     = "Global Comment"
	sidebarHint        = "Click a line number, or the + beside it, to leave feedback on a line."
	annotationsCountOf = "annotations"
)

func (t *Tab) sidebar(c *ui.Context) {
	p := token.Of(c)
	ui.Column(c).Fill().Background(p.Layer.SideBar).Role(ui.RoleRegion).Label(sidebarTitle).Children(func() {
		ui.Row(c).FillWidth().Height(token.ToolbarHeight.Px()).Shrink(0).Padding(0, token.PadPanel.Px()).Gap(token.GapInline.Px()).AlignItems(ui.Center).
			BorderWidth(0, 0, token.BorderWidth.Px(), 0).BorderColor(p.Border.Default).Children(func() {
			design.Text(c, sidebarTitle, token.TextStrong).Role(ui.RoleHeading).Level(2)
			badge.Count(c, len(t.notes), badge.CountOptions{Label: annotationsCountOf})
		})
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(token.PadPanel.Px()).Gap(token.GapGroup.Px()).Children(func() {
				t.globalSection(c)
				t.fileSections(c)
				if len(t.notes) == 0 && !t.inline.open {
					design.Text(c, sidebarHint, token.TextParagraph).Wrap()
				}
			})
		})
	})
}

func (t *Tab) globalSection(c *ui.Context) {
	ui.Column(c.Key("global")).FillWidth().Gap(token.GapInline.Px()).Children(func() {
		design.Text(c, globalTitle, token.TextTitle).Role(ui.RoleHeading).Level(3)
		for _, n := range t.notes {
			if n.Scope == review.ScopeGeneral && !t.editingPanel(n.id) {
				t.card(c, n, true)
			}
		}
		if t.panel.open && t.panel.ann.Scope == review.ScopeGeneral {
			t.composer(c, &t.panel, globalComposer)
			return
		}
		add, _ := action.Lookup(AddGlobalCommentID)
		if action.Button(c, addGlobalLabel, action.ButtonOptions{Icon: icons.Plus, Small: true, Keys: add.Keys, ID: AddGlobalCommentID}).Clicked() {
			t.AddGlobalComment()
		}
	})
}

func (t *Tab) editingPanel(id int) bool { return t.panel.open && t.panel.editing == id }

func (t *Tab) fileSections(c *ui.Context) {
	var paths []string
	seen := map[string]bool{}
	add := func(path string) {
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	for _, n := range t.notes {
		if n.Scope != review.ScopeGeneral {
			add(n.Path)
		}
	}
	if t.panel.open && t.panel.ann.Scope == review.ScopeFile {
		add(t.panel.ann.Path)
	}
	for _, path := range paths {
		ui.Column(c.Key("file:" + path)).FillWidth().Gap(token.GapInline.Px()).Children(func() {
			design.Text(c, path, token.TextStrong).SingleLine().Role(ui.RoleHeading).Level(3)
			for _, scope := range []review.Scope{review.ScopeFile, review.ScopeLine} {
				for _, n := range t.notes {
					if n.Path == path && n.Scope == scope && !t.editingPanel(n.id) {
						t.card(c, n, true)
					}
				}
			}
			if t.panel.open && t.panel.ann.Scope == review.ScopeFile && t.panel.ann.Path == path {
				t.composer(c, &t.panel, fileCommentLabel)
			}
		})
	}
}
