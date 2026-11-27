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

func toolbarGroup(c *ui.Context, children func()) {
	ui.Row(c).Height(token.ControlHeight.Px()).Gap(token.GapTight.Px()).AlignItems(ui.Center).Shrink(0).Children(children)
}

func toolbarSeparator(c *ui.Context) { pane.Separator(c, true) }

func (v *View) toolbar(c *ui.Context, o Options) {
	p := token.Of(c)
	ui.Row(c).FillWidth().MinHeight(token.ToolbarHeight.Px()).Shrink(0).Padding(token.GapInline.Px(), token.PadRow.Px()).Gap(token.GapTight.Px()).AlignItems(ui.Center).Wrap().
		BorderWidth(0, 0, token.BorderWidth.Px(), 0).BorderColor(p.Border.Default).
		Role(ui.RoleToolbar).Label("Diff Toolbar").FocusGroup(ui.Horizontal).Children(func() {
		toolbarGroup(c, func() {
			shown := v.treeShown()
			if action.ToggleIconButton(c, icons.Sidebar, "Toggle File Tree", &shown, action.IconOptions{ID: "toggle-tree"}).Changed() {
				v.tree.shown, v.tree.forced = shown, shown
			}
			files, adds, dels := v.summary()
			ui.Row(c).Gap(token.GapRowIcon.Px()).AlignItems(ui.Center).Shrink(0).Margin(0, 0, 0, token.GapInline.Px()).Children(func() {
				noun := "files"
				if files == 1 {
					noun = "file"
				}
				design.Text(c, fmt.Sprintf("%d %s", files, noun), token.TextLabel).SingleLine().Shrink(0)
				filediff.Counts(c, adds, dels)
			})
			if o.Busy {
				feedback.Spinner(c, "Loading diff")
			}
		})
		design.Spacer(c)
		toolbarGroup(c, func() {
			action.IconButton(c, icons.ChevronUp, "Previous Change", action.IconOptions{Keys: keyPrevChange, ID: "prev-change"}).OnClick(func() { v.step(-1) })
			action.IconButton(c, icons.ChevronDown, "Next Change", action.IconOptions{Keys: keyNextChange, ID: "next-change"}).OnClick(func() { v.step(1) })
			action.IconButton(c, icons.ArrowUp, "Previous File", action.IconOptions{Keys: keyPrevFile, Disabled: !v.hasFile(-1), ID: "prev-file"}).OnClick(func() { v.stepFile(-1) })
			action.IconButton(c, icons.ArrowDown, "Next File", action.IconOptions{Keys: keyNextFile, Disabled: !v.hasFile(1), ID: "next-file"}).OnClick(func() { v.stepFile(1) })
		})
		toolbarSeparator(c)
		toolbarGroup(c, func() {
			action.IconButton(c, icons.CollapseAll, "Collapse All", action.IconOptions{ID: "collapse-all"}).OnClick(func() { v.setAll(false) })
			action.IconButton(c, icons.ExpandAll, "Expand All", action.IconOptions{ID: "expand-all"}).OnClick(func() { v.setAll(true) })
		})
		toolbarSeparator(c)
		toolbarGroup(c, func() { action.Segmented(c, "Diff Layout", &v.layout, layoutNames, action.SegmentedOptions{}) })
		toolbarSeparator(c)
		toolbarGroup(c, func() { diffmode.Controls(c, v.engine()) })
		if o.Toolbar != nil {
			toolbarSeparator(c)
			toolbarGroup(c, func() { o.Toolbar(c) })
		}
	})
}
