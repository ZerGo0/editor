package gitui

import (
	"strings"

	"github.com/ZerGo0/editor/internal/design/quickinput"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

type branchesState struct {
	open      bool
	query     string
	input     quickinput.State
	page      *branchEntry
	actions   quickinput.State
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
	s.open, s.query, s.page = true, "", nil
	s.input, s.actions = quickinput.State{}, quickinput.State{}
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

type popupCommand struct {
	label string
	icon  *ui.SVG
	run   func()
}

func (g *Git) popupCommands() []popupCommand {
	s := &g.branches
	if s.pick != nil || strings.TrimSpace(s.query) != "" {
		return nil
	}
	return []popupCommand{
		{label: "New Branch...", icon: icons.Plus, run: func() { g.newBranch("") }},
		{label: "Checkout Tag or Revision...", icon: icons.Tag, run: g.checkoutRevision},
		{label: "Manage Remotes...", icon: icons.Settings, run: g.openRemotes},
		{label: "Fetch", icon: icons.Fetch, run: g.fetch},
	}
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
	if s.page != nil {
		g.branchPage(c, *s.page)
		return
	}
	cmds := g.popupCommands()
	entries := buildBranchEntries(g.snap.branches, s.tags, s.query)
	items := make([]quickinput.Item, 0, len(cmds)+len(entries))
	refs := make([]int, 0, len(entries))
	for _, cmd := range cmds {
		items = append(items, quickinput.Item{Label: cmd.label, Icon: cmd.icon})
	}
	group := ""
	for i, e := range entries {
		if e.header {
			group = e.title
			continue
		}
		items = append(items, branchItem(e, group))
		refs = append(refs, i)
	}
	empty := "No matching branches"
	if s.tagsErr != nil {
		empty = "Cannot load tags: " + s.tagsErr.Error()
	}
	placeholder := "Select a branch or tag to check out"
	if s.pick != nil {
		placeholder = s.pickTitle
	}
	res := quickinput.View(c, s.pickTitle, &s.input, items, quickinput.Options{Placeholder: placeholder, Busy: g.busy != "", Empty: empty})
	s.query = s.input.Query
	switch {
	case res.Closed:
		s.open = false
	case res.Accepted >= 0 && res.Accepted < len(cmds):
		s.open = false
		cmds[res.Accepted].run()
	case res.Accepted >= len(cmds):
		g.chooseEntry(entries[refs[res.Accepted-len(cmds)]])
	}
}

func branchItem(e branchEntry, group string) quickinput.Item {
	it := quickinput.Item{Label: e.name, Group: group, Icon: icons.GitBranch}
	var details []string
	switch {
	case e.kind == kindTag:
		it.Icon = icons.Tag
	case e.branch.Current:
		it.Icon = icons.Check
	}
	if s := syncLabel(e.branch.Ahead, e.branch.Behind); s != "" {
		details = append(details, s)
	}
	if e.branch.Upstream != "" {
		details = append(details, e.branch.Upstream)
	}
	if e.branch.Gone {
		details = append(details, "gone")
	}
	it.Description = strings.Join(details, " \u2022 ")
	return it
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
	s.page, s.actions = &e, quickinput.State{}
}

func (g *Git) branchPage(c *ui.Context, e branchEntry) {
	s := &g.branches
	var acts []branchAction
	for _, a := range g.branchActions(e) {
		if !a.separator && !a.disabled {
			acts = append(acts, a)
		}
	}
	query := strings.ToLower(strings.TrimSpace(s.actions.Query))
	var shown []branchAction
	items := make([]quickinput.Item, 0, len(acts))
	for _, a := range acts {
		if query != "" && !strings.Contains(strings.ToLower(a.label), query) {
			continue
		}
		shown = append(shown, a)
		items = append(items, quickinput.Item{Label: a.label})
	}
	res := quickinput.View(c, e.name, &s.actions, items, quickinput.Options{Placeholder: "Choose an action for " + e.name})
	switch {
	case res.Closed:
		s.page = nil
	case res.Accepted >= 0:
		s.open = false
		shown[res.Accepted].run()
	}
}
