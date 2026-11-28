package gitui

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/git/diffmode"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type diffOptions struct {
	layout        int
	layoutSet     bool
	inline        editor.DiffInline
	expandAll     bool
	hideNumbers   bool
	whitespaceSet bool
	movesSet      bool
}

var inlineModes = []struct {
	mode  editor.DiffInline
	label string
}{
	{editor.DiffInlineWordAlt, "Highlight Words"},
	{editor.DiffInlineWordLine, "Highlight Changed Span"},
	{editor.DiffInlineChar, "Highlight Characters"},
	{editor.DiffInlineNone, "Highlight Lines Only"},
}

func (o diffOptions) apply(d *editor.DiffState) {
	d.Inline = o.inline
	d.ExpandUnchanged = o.expandAll
	d.NoLineNumbers = o.hideNumbers
}

func (g *Git) diffOptionsButton(c *ui.Context) {
	action.MenuIconButton(c, icons.More, "Diff Options", action.IconOptions{}, func(m *ui.Menu) {
		o := &g.diffOpts
		for _, im := range inlineModes {
			if m.Item(im.label).Radio(o.inline == im.mode).Chosen() {
				o.inline = im.mode
				g.saveDiffPrefs()
			}
		}
		m.Separator()
		if m.Item("Collapse Unchanged Fragments").Checked(!o.expandAll).Chosen() {
			o.expandAll = !o.expandAll
			g.saveDiffPrefs()
		}
		var configured editor.DiffState
		g.h.ConfigureDiff(&configured, "")
		if m.Item("Wrap Lines").Checked(configured.Wrap).Disabled(g.toggleWrap == nil).Chosen() {
			g.toggleWrap()
		}
		if m.Item("Show Line Numbers").Checked(!o.hideNumbers).Chosen() {
			o.hideNumbers = !o.hideNumbers
			g.saveDiffPrefs()
		}
	})
}

func diffModeControls(c *ui.Context, e *diffmode.Engine) {
	options := make([]action.Option[git.WhitespaceMode], 0, len(diffmode.WhitespaceChoices))
	for _, choice := range diffmode.WhitespaceChoices {
		options = append(options, action.Option[git.WhitespaceMode]{Value: choice.Mode, Label: choice.Label})
	}
	action.Select(c, "Ignore Whitespace", &diffmode.Session.Whitespace, options, action.SelectOptions{ID: "diff-whitespace"})
	action.Checkbox(c, "Show Moved Code", &diffmode.Session.ShowMoves, action.ChoiceOptions{ID: "diff-show-moves"}).MinHeight(token.HitTargetMin.Px())
	if e == nil {
		return
	}
	if e.Busy() {
		feedback.Spinner(c, "Comparing")
	}
	if e.Notice() != "" {
		token.TextLabel.Apply(ui.Text(c, e.Notice())).TextColor(token.Of(c).Severity.Warning).SingleLine().Shrink(1).MinWidth(0)
	}
}
