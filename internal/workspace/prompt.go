package workspace

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

const promptWidth = 360

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
	t := c.Theme()
	ui.Modal(c, &p.open, func() {
		ui.Column(c).Gap(10).Width(promptWidth).Children(func() {
			ui.Text(c, p.title).Bold()
			input := ui.TextInput(c, &p.value).AutoFocus().FillWidth().Label(p.title)
			if p.err != "" {
				ui.Text(c, p.err).TextColor(t.Danger)
			}
			var ok, cancel ui.Element
			ui.Row(c).Gap(8).Justify(ui.End).FillWidth().Children(func() {
				cancel = ui.Button(c, "Cancel")
				ok = ui.PrimaryButton(c, p.action)
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
	})
}
