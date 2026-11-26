package gitui

import (
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	popupTop    = 60
	popupWidth  = 460
	popupRadius = 8
	popupPad    = 6
)

type branchesState struct {
	open      bool
	query     string
	sel       int
	list      ui.ListState
	page      *branchEntry
	actSel    int
	actList   ui.ListState
	tags      []git.Tag
	tagsErr   error
	gen       int
	pick      func(ref string)
	pickTitle string
}

func (g *Git) openBranches() {
	g.openBranchPopup("Branches", nil)
}

func (g *Git) openBranchPicker(title string, pick func(ref string)) {
	g.openBranchPopup(title, pick)
}

func (g *Git) openBranchPopup(title string, pick func(ref string)) {
	if !g.ready() {
		return
	}
	s := &g.branches
	s.open, s.query, s.page, s.sel, s.actSel = true, "", nil, -1, 0
	s.pick, s.pickTitle = pick, title
	g.refreshNow()
	g.loadTags()
}

func (g *Git) loadTags() {
	s := &g.branches
	s.gen++
	gen, r := s.gen, g.repo
	go func() {
		tags, err := r.Tags(g.ctx)
		g.post(func() {
			if gen == s.gen {
				s.tags, s.tagsErr = tags, err
			}
		})
	}()
}

func (g *Git) branchesPopup(c *ui.Context) {
	s := &g.branches
	if !s.open {
		return
	}
	if !g.ready() {
		s.open = false
		return
	}
	t := c.Theme()
	entries := buildBranchEntries(g.snap.branches, s.tags, s.query)
	if s.sel < 0 || s.sel >= len(entries) || entries[s.sel].header {
		s.sel = nextEntry(entries, -1, 1)
	}
	ui.Overlay(c, func() {
		back := ui.Column(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).AlignItems(ui.Center).Modal()
		back.Children(func() {
			panel := ui.Column(c).Margin(popupTop, 0, 0, 0).Width(popupWidth).Padding(popupPad).Gap(4).
				Background(t.Background).Radius(popupRadius).BorderWidth(1).BorderColor(t.Border).
				Role(ui.RoleDialog).Label(s.pickTitle)
			panel.Children(func() {
				if s.page != nil {
					g.branchPage(c, *s.page)
					return
				}
				g.branchList(c, entries)
			})
			escape := back.OverlayShortcut(0, ui.KeyEscape)
			switch {
			case escape && s.page != nil:
				s.page = nil
			case escape, panel.PressedOutside():
				s.open = false
			}
		})
	})
}

func (g *Git) branchList(c *ui.Context, entries []branchEntry) {
	s := &g.branches
	ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Center).Children(func() {
		input := ui.TextInput(c.Key("branch-query"), &s.query).AutoFocus().Grow(1).MinWidth(0).
			Placeholder("Search branches and tags").Label("Search Branches")
		switch {
		case input.Shortcut(0, ui.KeyDown):
			s.sel = nextEntry(entries, s.sel, 1)
			s.list.ScrollIntoView(s.sel)
		case input.Shortcut(0, ui.KeyUp):
			s.sel = nextEntry(entries, s.sel, -1)
			s.list.ScrollIntoView(s.sel)
		}
		if input.Submitted() && s.sel >= 0 && s.sel < len(entries) {
			g.chooseEntry(entries[s.sel])
		}
		if s.pick == nil {
			if iconButton(c, "Fetch", icons.Fetch, g.busy != "").Clicked() {
				g.fetch()
			}
		}
	})
	if s.pick == nil {
		ui.Row(c).FillWidth().Gap(2).Wrap().Children(func() {
			if textButton(c, "New Branch...", false).Clicked() {
				s.open = false
				g.newBranch("")
			}
			if textButton(c, "Checkout Tag or Revision...", g.busy != "").Clicked() {
				s.open = false
				g.checkoutRevision()
			}
			if textButton(c, "Manage Remotes...", false).Clicked() {
				s.open = false
				g.openRemotes()
			}
		})
	}
	if s.tagsErr != nil {
		ui.Text(c, "Cannot load tags: "+s.tagsErr.Error()).FontSize(smallFont).TextColor(c.Theme().Danger)
	}
	if len(entries) == 0 {
		emptyState(c, "No matching branches", "")
		return
	}
	s.list.Key = func(i int) any { return entries[i].key() }
	s.list.Label = func(i int) string {
		if entries[i].header {
			return entries[i].title
		}
		return entries[i].name
	}
	s.list.Header = func(i int) bool { return entries[i].header }
	s.list.Selected = &s.sel
	list := ui.List(c.Key("branch-list"), &s.list, len(entries)).FillWidth().Height(listHeight).Label("Branches")
	list.Rows(func(row ui.ListRow) { g.branchRow(row, entries[row.Index]) })
	if list.Submitted() && s.sel >= 0 && s.sel < len(entries) {
		g.chooseEntry(entries[s.sel])
	}
}

func (g *Git) branchRow(row ui.ListRow, e branchEntry) {
	c := row.Context
	t := c.Theme()
	if e.header {
		ui.Row(c).Height(rowHeight).FillWidth().Padding(0, rowPadding).AlignItems(ui.Center).Children(func() {
			sectionTitle(c, e.title)
		})
		return
	}
	el := ui.Row(c).Height(rowHeight).FillWidth().Padding(0, rowPadding).Gap(6).AlignItems(ui.Center)
	el.Children(func() {
		icon := icons.GitBranch
		if e.kind == kindTag {
			icon = icons.Tag
		}
		color := t.TextMuted
		if e.branch.Current {
			color = t.Accent
		}
		ui.Icon(c, icon).Size(iconSize, iconSize).TextColor(color).Shrink(0)
		name := ui.Text(c, e.name).FontSize(bodyFont).SingleLine().Shrink(1).MinWidth(0)
		if e.branch.Current {
			name.Bold()
		}
		if s := syncLabel(e.branch.Ahead, e.branch.Behind); s != "" {
			muted(c, s).SingleLine().Shrink(0)
		}
		if e.branch.Gone {
			ui.Text(c, "gone").FontSize(smallFont).TextColor(t.Warning).SingleLine().Shrink(0)
		}
		ui.Box(c).Grow(1)
		if e.branch.Upstream != "" {
			muted(c, e.branch.Upstream).SingleLine().Shrink(1).MinWidth(0)
		}
	})
	if el.RowClicked() {
		g.chooseEntry(e)
	}
	if g.branches.pick == nil {
		el.ContextMenu(func(m *ui.Menu) {
			for _, a := range g.branchActions(e) {
				if a.separator {
					m.Separator()
					continue
				}
				it := m.Item(a.label).Disabled(a.disabled)
				if a.danger {
					it.Danger()
				}
				if it.Chosen() {
					g.branches.open = false
					a.run()
				}
			}
		})
	}
}

func (g *Git) chooseEntry(e branchEntry) {
	s := &g.branches
	if e.header {
		return
	}
	if s.pick != nil {
		s.open = false
		s.pick(e.name)
		return
	}
	s.page, s.actSel = &e, firstAction(g.branchActions(e))
}

func firstAction(acts []branchAction) int {
	for i, a := range acts {
		if !a.separator && !a.disabled {
			return i
		}
	}
	return 0
}

func (g *Git) branchPage(c *ui.Context, e branchEntry) {
	s := &g.branches
	acts := g.branchActions(e)
	ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
		if iconButton(c, "Back", icons.ChevronLeft, false).Clicked() {
			s.page = nil
		}
		ui.Text(c, e.name).FontSize(bodyFont).Bold().SingleLine().Shrink(1).MinWidth(0)
	})
	s.actList.Label = func(i int) string { return acts[i].label }
	s.actList.Header = func(i int) bool { return acts[i].separator }
	s.actList.Selected = &s.actSel
	list := ui.List(c.Key("branch-actions"), &s.actList, len(acts)).FillWidth().Height(listHeight).AutoFocus().
		Label("Branch Actions")
	list.Rows(func(row ui.ListRow) {
		rc := row.Context
		a := acts[row.Index]
		if a.separator {
			ui.Divider(rc).Margin(3, 0)
			return
		}
		el := ui.Row(rc).Height(rowHeight).FillWidth().Padding(0, rowPadding).AlignItems(ui.Center).Children(func() {
			text := ui.Text(rc, a.label).FontSize(bodyFont).SingleLine()
			switch {
			case a.disabled:
				text.TextColor(rc.Theme().TextMuted)
			case a.danger:
				text.TextColor(rc.Theme().Danger)
			}
		})
		if el.RowClicked() && !a.disabled {
			s.open = false
			a.run()
		}
	})
	if list.Submitted() && s.actSel >= 0 && s.actSel < len(acts) && !acts[s.actSel].disabled && !acts[s.actSel].separator {
		s.open = false
		acts[s.actSel].run()
	}
}
