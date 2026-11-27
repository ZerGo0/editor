package workspace

import (
	"strings"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/dialog"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/egoist/mygo/ui"
)

const cancelLabel = "Cancel"

type prompt struct {
	open   bool
	title  string
	action string
	value  string
	err    string
	submit func(string) error
}

func (w *Workspace) ask(title, action, value string, submit func(string) error) {
	w.prompt = prompt{open: true, title: title, action: action, value: value, submit: submit}
}

func (w *Workspace) promptView(c *ui.Context) {
	p := &w.prompt
	if !p.open {
		return
	}
	dialog.Modal(c, &p.open, p.title, dialog.Options{}, func() {
		input := field.TextInput(c, p.title, &p.value, field.InputOptions{AutoFocus: true, Error: p.err})
		var ok, cancel ui.Element
		dialog.Actions(c, func() {
			cancel = action.Button(c, cancelLabel, action.ButtonOptions{})
			ok = action.Button(c, p.action, action.ButtonOptions{Variant: action.Primary})
		})
		switch {
		case cancel.Clicked():
			p.open = false
		case input.Submitted() || ok.Clicked():
			if err := p.submit(strings.TrimSpace(p.value)); err != nil {
				p.err = err.Error()
				return
			}
			p.open = false
		}
	})
}
