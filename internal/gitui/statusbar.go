package gitui

import (
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

func (g *Git) StatusItems(c *ui.Context) {
	if !g.ready() || g.snap.status == nil {
		return
	}
	t := c.Theme()
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		if g.busy != "" {
			ui.Spinner(c).Size(iconSize, iconSize).Label(g.busy)
			muted(c, g.busy+"...").SingleLine()
		}
		if st := g.snap.op; st.Kind != git.StateNone {
			title := opTitle(st)
			b := ui.ButtonBase(c.Key("op")).Height(iconButtonSize).Padding(0, 6).Radius(buttonRadius).AlignItems(ui.Center).
				Label("Git Operation " + title).Tooltip(title)
			b.Background(t.Warning.Alpha(tintAlpha))
			b.Children(func() { ui.Text(c, title).FontSize(smallFont).SingleLine() })
			b.PopoverMenuButton(g.operationMenu)
		}
		name := g.headLabel()
		b := ui.ButtonBase(c.Key("branch")).Height(iconButtonSize).Padding(0, 6).Gap(4).Radius(buttonRadius).AlignItems(ui.Center).
			Label("Git Branch " + name).Tooltip("Branches")
		if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
		b.Children(func() {
			ui.Icon(c, icons.GitBranch).Size(iconSize, iconSize)
			ui.Text(c, name).FontSize(smallFont).SingleLine()
			if s := syncLabel(g.snap.status.Ahead, g.snap.status.Behind); s != "" {
				muted(c, s).SingleLine()
			}
		})
		b.OnClick(g.openBranches)
	})
}

func (g *Git) operationMenu(m *ui.Menu) {
	busy := g.busy != ""
	conflicts := g.conflictCount()
	if m.Item("Resolve Conflicts...").Disabled(conflicts == 0).Chosen() {
		g.openConflicts()
	}
	if m.Item("Continue").Disabled(busy || conflicts > 0).Chosen() {
		g.continueOp()
	}
	if m.Item("Skip").Disabled(busy || g.snap.op.Kind == git.StateMerging).Chosen() {
		g.skipOp()
	}
	if m.Item("Abort").Danger().Disabled(busy).Chosen() {
		g.abortOp()
	}
}
