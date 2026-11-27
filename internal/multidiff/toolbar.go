package multidiff

import (
	"fmt"

	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/filediff"
	"github.com/ZerGo0/editor/internal/design/pane"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git/diffmode"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	keyNextChange = "F7"
	keyPrevChange = "\u21e7F7"
	keyNextFile   = "\u2325F5"
	keyPrevFile   = "\u21e7\u2325F5"
)

var layoutNames = []string{"Side by Side", "Unified"}

func (v *View) toolbar(c *ui.Context, o Options) {
	ui.Row(c).FillWidth().MinHeight(token.ToolbarHeight.Px()).Shrink(0).Padding(token.GapInline.Px(), token.PadRow.Px()).Gap(token.GapTight.Px()).AlignItems(ui.Center).Wrap().
		Role(ui.RoleToolbar).Label("Diff Toolbar").FocusGroup(ui.Horizontal).Children(func() {
		shown := v.treeShown()
		if action.ToggleIconButton(c, icons.Sidebar, "Toggle File Tree", &shown, action.IconOptions{ID: "toggle-tree"}).Changed() {
			v.tree.shown, v.tree.forced = shown, shown
		}
		files, adds, dels := v.summary()
		design.Text(c, fmt.Sprintf("%d files", files), token.TextLabel).SingleLine().Shrink(0)
		filediff.Counts(c, adds, dels)
		if o.Busy {
			feedback.Spinner(c, "Loading diff")
		}
		design.Spacer(c)
		action.IconButton(c, icons.ChevronUp, "Previous Change", action.IconOptions{Keys: keyPrevChange, ID: "prev-change"}).OnClick(func() { v.step(-1) })
		action.IconButton(c, icons.ChevronDown, "Next Change", action.IconOptions{Keys: keyNextChange, ID: "next-change"}).OnClick(func() { v.step(1) })
		action.IconButton(c, icons.ArrowUp, "Previous File", action.IconOptions{Keys: keyPrevFile, Disabled: !v.hasFile(-1), ID: "prev-file"}).OnClick(func() { v.stepFile(-1) })
		action.IconButton(c, icons.ArrowDown, "Next File", action.IconOptions{Keys: keyNextFile, Disabled: !v.hasFile(1), ID: "next-file"}).OnClick(func() { v.stepFile(1) })
		pane.Separator(c, true)
		action.IconButton(c, icons.CollapseAll, "Collapse All", action.IconOptions{ID: "collapse-all"}).OnClick(func() { v.setAll(false) })
		action.IconButton(c, icons.ExpandAll, "Expand All", action.IconOptions{ID: "expand-all"}).OnClick(func() { v.setAll(true) })
		pane.Separator(c, true)
		action.Segmented(c, "Diff Layout", &v.layout, layoutNames, action.SegmentedOptions{})
		var eng *diffmode.Engine
		if f := v.current(); f != nil {
			eng = f.engine
		}
		diffmode.Controls(c, eng)
		if o.Toolbar != nil {
			o.Toolbar(c)
		}
	})
}
