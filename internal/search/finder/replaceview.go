package finder

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/dialog"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/picker"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/findwidget"
	"github.com/egoist/mygo/ui"
)

const (
	titleReplace       = "Replace in Files"
	replaceLabel       = "Replace"
	replaceAllLabel    = "Replace All"
	replaceFileLabel   = "Replace All in File"
	replaceKeys        = "⇧⌘1"
	toggleReplaceLabel = "Toggle Replace"
	maskLabel          = "File mask"
	maskPlaceholder    = "File mask, for example *.go, *.md"
	scopeSelectLabel   = "Scope"
	directoryLabel     = "Directory"
	directoryHint      = "Directory path"
	confirmHint        = "Open files change in their editors and stay unsaved. Other files are written to disk."
	cancelLabel        = "Cancel"
	replaceConfirmID   = "replace-confirm"
	replaceCancelID    = "replace-cancel"
)

func (d *Dialog) queryRow(c *ui.Context, label string, toggles []field.Toggle) picker.Keys {
	var keys picker.Keys
	ui.Row(c).FillWidth().AlignItems(ui.Start).Gap(token.GapInline.Px()).Children(func() {
		if d.tab == TabText {
			if action.ExpandButton(c, toggleReplaceLabel, d.st.ShowReplace, action.IconOptions{KeepFocus: true}).Clicked() {
				d.st.ShowReplace = !d.st.ShowReplace
			}
		}
		ui.Column(c.Key("search-fields")).Grow(1).MinWidth(0).Gap(token.GapInline.Px()).Children(func() {
			keys = picker.Query(c, label, &d.st.Query, picker.QueryOptions{Placeholder: d.placeholder(), Toggles: toggles, Error: d.queryError(),
				Busy: d.showBusy, Handle: &d.field, NoAutoFocus: d.focusWith, Multiline: d.tab == TabText})
			if d.replaceVisible() {
				d.replaceRow(c)
			}
		})
	})
	return keys
}

func (d *Dialog) replaceRow(c *ui.Context) {
	ui.Row(c.Key("replace-row")).FillWidth().AlignItems(ui.Start).Gap(token.GapInline.Px()).Children(func() {
		in, submit := findwidget.Field(c, replaceLabel, &d.st.With, findwidget.FieldOptions{Toggles: []field.Toggle{findwidget.PreserveCase(&d.st.PreserveCase)}})
		in.Grow(1).MinWidth(0)
		if d.focusWith {
			in.Focus()
			d.focusWith = !in.Focused()
		}
		switch submit {
		case findwidget.CmdEnter:
			d.askReplaceAll()
		case findwidget.Enter:
			d.replaceSelected()
		}
		ui.Row(c).Height(token.ControlHeight.Px()).AlignItems(ui.Center).Shrink(0).Children(func() {
			if findwidget.ReplaceAll.Clicked(c, !d.canReplace()) {
				d.askReplaceAll()
			}
		})
	})
}

func (d *Dialog) optionsRow(c *ui.Context) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(token.GapControl.Px()).Children(func() {
		field.TextInput(c, maskLabel, &d.st.Mask, field.InputOptions{Placeholder: maskPlaceholder}).Grow(1).MinWidth(0)
		scopes := []action.Option[string]{{Value: ScopeProject, Label: ScopeProject}, {Value: ScopeDirectory, Label: ScopeDirectory}}
		if d.st.Scope == "" {
			d.st.Scope = ScopeProject
		}
		action.Select(c, scopeSelectLabel, &d.st.Scope, scopes, action.SelectOptions{})
		if d.st.Scope == ScopeDirectory {
			field.TextInput(c, directoryLabel, &d.st.Directory, field.InputOptions{Placeholder: directoryHint}).Grow(1).MinWidth(0)
		}
	})
}

func (d *Dialog) confirmView(c *ui.Context) {
	if !d.confirm {
		return
	}
	hint := "Replace " + plural(d.text.total, "occurrence") + " across " + plural(len(d.text.files), "file") + "? " + confirmHint
	dialog.Modal(c, &d.confirm, replaceAllLabel, dialog.Options{Hint: hint}, func() {
		var cancel, ok ui.Element
		dialog.Actions(c, func() {
			cancel = action.Button(c, cancelLabel, action.ButtonOptions{ID: replaceCancelID})
			ok = action.Button(c, replaceAllLabel, action.ButtonOptions{Variant: action.Primary, ID: replaceConfirmID})
		})
		switch {
		case cancel.Clicked():
			d.confirm = false
		case ok.Clicked():
			d.confirm = false
			d.replaceAll()
		}
	})
}
