package focus

import "github.com/egoist/mygo/ui"

type Keys struct {
	Observe func(ev ui.InputEvent)
	Handle  func(c *ui.Context)
}

func (k Keys) Watch(back ui.Element) ui.Element {
	if k.Observe == nil {
		return back
	}
	return back.ObserveInput(k.Observe)
}

func (k Keys) Run(c *ui.Context) {
	if k.Handle != nil {
		k.Handle(c)
	}
}
