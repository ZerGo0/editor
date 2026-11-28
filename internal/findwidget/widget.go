package findwidget

import (
	"errors"
	"regexp/syntax"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/pane"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	findLabel          = "Find"
	replaceLabel       = "Replace"
	toggleReplaceLabel = "Toggle Replace"
	noResults          = "No results"
	invalidRegexp      = "Invalid regular expression"
	numericFeatures    = "tnum"
	resizeLabel        = "Resize Find"
)

var (
	matchCase = field.Toggle{Icon: icons.CaseSensitive, Label: "Match Case", Keys: "⌥⌘C"}
	wholeWord = field.Toggle{Icon: icons.WholeWord, Label: "Match Whole Word", Keys: "⌥⌘W"}
	useRegexp = field.Toggle{Icon: icons.Regex, Label: "Use Regular Expression", Keys: "⌥⌘R"}
	keepCase  = field.Toggle{Icon: icons.PreserveCase, Label: "Preserve Case", Keys: "⌥⌘P"}

	toggleKeys = map[string]ui.Key{
		matchCase.Label: ui.KeyC, wholeWord.Label: ui.KeyW, useRegexp.Label: ui.KeyR, keepCase.Label: ui.KeyP, FindInSelection.Label: ui.KeyL,
	}
)

var ToggleMods = ui.Alt | ui.Cmd

func ToggleKey(label string) ui.Key { return toggleKeys[label] }

func Toggles(caseOn, wordOn, regexOn *bool) []field.Toggle {
	toggles := []field.Toggle{matchCase, wholeWord, useRegexp}
	for i, on := range []*bool{caseOn, wordOn, regexOn} {
		toggles[i].On = on
	}
	return toggles
}

func PreserveCase(on *bool) field.Toggle {
	t := keepCase
	t.On = on
	return t
}

func Render(c *ui.Context, bar ui.Element, f *editor.FindBar) { build(c, bar, f, nil, nil) }

func ForEditor(st *editor.State, sh *Shared) func(c *ui.Context, bar ui.Element, f *editor.FindBar) {
	return func(c *ui.Context, bar ui.Element, f *editor.FindBar) { build(c, bar, f, st, sh) }
}

func build(c *ui.Context, bar ui.Element, f *editor.FindBar, st *editor.State, sh *Shared) {
	p := token.Of(c)
	sh.apply(f)
	toggleShortcuts(bar, f, sh)
	bar.Row().AlignItems(ui.Stretch).Gap(token.GapTight.Px()).Padding(token.PadPopover.Px()).Background(p.Layer.Widget).
		Border(token.BorderWidth.Px(), p.Border.Widget).Radius(token.RadiusPopover.Px()).TextColor(p.Text.Primary)
	token.ElevationPopover.Apply(bar, p)
	bar.Children(func() {
		replacing := f.Replacing()
		if action.ExpandButton(c, toggleReplaceLabel, replacing, action.IconOptions{Disabled: !f.CanReplace(), KeepFocus: true}).Clicked() {
			f.SetReplacing(!replacing)
		}
		var input ui.Element
		ui.Column(c).Gap(token.GapInline.Px()).Shrink(1).MinWidth(0).Children(func() {
			input = findRow(c, p, f, st, sh)
			if replacing {
				replaceRow(c, f, sh)
			}
		})
		if sh != nil {
			sash, r := pane.Sash(c, pane.SashOptions{Label: resizeLabel})
			sash.Left(0).Top(0).Bottom(0)
			sh.resize(input.Bounds().W, r)
		}
	})
	sh.apply(f)
}

func toggleShortcuts(bar ui.Element, f *editor.FindBar, sh *Shared) {
	toggles := Toggles(options(f, sh))
	toggles = append(toggles, field.Toggle{Label: FindInSelection.Label, On: f.InSelection()})
	for _, t := range toggles {
		if bar.Shortcut(ToggleMods, ToggleKey(t.Label)) {
			*t.On = !*t.On
		}
	}
}

func findRow(c *ui.Context, p *token.Palette, f *editor.FindBar, st *editor.State, sh *Shared) ui.Element {
	count, _ := f.Matches()
	return fieldRow(c, sh, func() {
		in, submit := Field(c, findLabel, f.Query(), FieldOptions{Error: regexpError(f.Err()), Toggles: Toggles(options(f, sh)), History: sh.history()})
		was := in.Focused()
		if sh != nil && in.Shortcut(0, ui.KeyEscape) {
			sh.remember(*f.Query())
			f.Close()
		}
		f.QueryInput(in)
		if st != nil && !was && in.Focused() {
			seedInSelection(st, f)
		}
		if submit != NoSubmit {
			sh.remember(*f.Query())
		}
		switch submit {
		case Enter:
			f.Next()
		case ShiftEnter:
			f.Previous()
		case AltEnter:
			f.SelectAll()
		}
	}, func() {
		status(c, p, f, count)
		if PreviousMatch.Clicked(c, count == 0) {
			sh.remember(*f.Query())
			f.Previous()
		}
		if NextMatch.Clicked(c, count == 0) {
			sh.remember(*f.Query())
			f.Next()
		}
		FindInSelection.Toggle(c, f.InSelection())
		if Close.Clicked(c, false) {
			sh.remember(*f.Query())
			f.Close()
		}
	})
}

func replaceRow(c *ui.Context, f *editor.FindBar, sh *Shared) {
	count, _ := f.Matches()
	fieldRow(c, sh, func() {
		in, submit := Field(c, replaceLabel, f.Replacement(), FieldOptions{})
		f.ReplacementInput(in)
		switch submit {
		case Enter:
			f.Replace()
		case CmdEnter:
			f.ReplaceAll()
		}
	}, func() {
		if ReplaceOne.Clicked(c, count == 0) {
			f.Replace()
		}
		if ReplaceAll.Clicked(c, count == 0) {
			f.ReplaceAll()
		}
	})
}

func fieldRow(c *ui.Context, sh *Shared, input, controls func()) ui.Element {
	var col ui.Element
	ui.Row(c).AlignItems(ui.Start).Gap(token.GapInline.Px()).Children(func() {
		col = ui.Column(c).Width(sh.inputWidth()).Shrink(1).MinWidth(0).Children(input)
		ui.Row(c).Height(token.ControlHeight.Px()).AlignItems(ui.Center).Gap(token.GapTight.Px()).Shrink(0).Children(controls)
	})
	return col
}

func status(c *ui.Context, p *token.Palette, f *editor.FindBar, count int) {
	text, color := f.Status(), p.Text.Primary
	switch {
	case *f.Query() == "":
		text, color = noResults, p.Text.Secondary
	case count == 0:
		text, color = noResults, p.Tone(token.ToneError)
	}
	token.TextLabel.Apply(ui.Text(c, text)).TextColor(color).SingleLine().FontFeatures(numericFeatures).
		MinWidth(token.FindStatusWidth.Px()).Role(ui.RoleStatus)
}

func regexpError(err error) string {
	if err == nil {
		return ""
	}
	var se *syntax.Error
	if errors.As(err, &se) {
		return invalidRegexp + ": " + se.Code.String()
	}
	return invalidRegexp
}

func seedInSelection(st *editor.State, f *editor.FindBar) {
	if *f.Query() != "" || *f.InSelection() {
		return
	}
	for _, s := range st.Selections() {
		if st.Buffer.Pos(s.Anchor).Line != st.Buffer.Pos(s.Head).Line {
			*f.InSelection() = true
			return
		}
	}
}
