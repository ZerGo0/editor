package gitui

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/dialog"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const cancelLabel = "Cancel"

func secondary(c *ui.Context, s string) ui.Element {
	return token.TextLabel.Apply(ui.Text(c, s)).TextColor(token.Of(c).Text.Secondary)
}

func paragraph(c *ui.Context, s string) ui.Element {
	return token.TextParagraph.Apply(ui.Text(c, s)).TextColor(token.Of(c).Text.Secondary).Wrap()
}

func errorText(c *ui.Context, s string) ui.Element {
	return token.TextLabel.Apply(ui.Text(c, s)).TextColor(token.Of(c).Severity.Error).Wrap().Role(ui.RoleAlert)
}

func emptyState(c *ui.Context, title, detail string) {
	feedback.Empty(c, feedback.EmptyOptions{Title: title, Message: detail})
}

func errorState(c *ui.Context, title string, err error, retry func()) {
	feedback.Error(c, title, describe(err), retry)
}

func button(c *ui.Context, label string, disabled bool) bool {
	return action.Button(c, label, action.ButtonOptions{Disabled: disabled}).Clicked() && !disabled
}

func primaryButton(c *ui.Context, label string, disabled bool) bool {
	return action.Button(c, label, action.ButtonOptions{Variant: action.Primary, Disabled: disabled}).Clicked() && !disabled
}

func smallButton(c *ui.Context, label string, disabled bool) bool {
	return action.Button(c, label, action.ButtonOptions{Small: true, Disabled: disabled}).Clicked() && !disabled
}

func iconButton(c *ui.Context, label string, svg *ui.SVG, disabled bool, keys ...string) bool {
	opts := action.IconOptions{Disabled: disabled}
	if len(keys) > 0 {
		opts.Keys = keys[0]
	}
	return action.IconClicked(c, svg, label, opts)
}

func dialogButtons(c *ui.Context, cancel func(), label string, disabled bool, ok func()) {
	dialog.Actions(c, func() {
		if button(c, cancelLabel, false) {
			cancel()
		}
		if primaryButton(c, label, disabled) {
			ok()
		}
	})
}

func modal(c *ui.Context, open *bool, title string, width token.Size, body func()) {
	if !*open {
		return
	}
	dialog.Modal(c, open, title, dialog.Options{Width: width}, body)
}

func boxedList(c *ui.Context, label string, s *ui.ListState, n int, item func(i int) list.Item) ui.Element {
	var l ui.Element
	boxFrame(c, func() { l = list.View(c, s, n, list.ViewOptions{Label: label}, item) })
	return l
}

func boxFrame(c *ui.Context, body func()) {
	bordered(c, ui.Column(c).FillWidth().Height(token.ListHeightMax.Px())).Children(body)
}

func bordered(c *ui.Context, el ui.Element) ui.Element {
	return el.Radius(token.RadiusControl.Px()).Clip().Border(token.BorderWidth.Px(), token.Of(c).Border.Input)
}

func changedFileItem(rel string, code byte, open func(preview, focus bool)) list.Item {
	name, dir := splitPath(rel)
	return list.Item{
		Label:         name,
		Description:   dir,
		Git:           gitStatus(code),
		Strike:        code == codeDeleted,
		Tooltip:       rel + " \u2022 " + statusName(code),
		Leading:       func(c *ui.Context) { icons.FileIcon(c, name, false, false, token.IconSize.Px()) },
		OnClick:       func() { open(true, false) },
		OnDoubleClick: func() { open(false, true) },
	}
}

func toolbarRow(c *ui.Context, label string, children func()) ui.Element {
	return sizedToolbarRow(c, label, token.ToolbarHeight.Px(), children)
}

func sizedToolbarRow(c *ui.Context, label string, height float32, children func()) ui.Element {
	p := token.Of(c)
	row := ui.Row(c).FillWidth().MinWidth(0).Height(height).Shrink(0).Padding(0, token.PadRow.Px()).Gap(token.GapControl.Px()).
		AlignItems(ui.Center).Role(ui.RoleToolbar).Label(label).Children(children)
	ui.Box(c).FillWidth().Height(token.BorderWidth.Px()).Shrink(0).Background(p.Border.Default).AccessHidden(true)
	return row
}
