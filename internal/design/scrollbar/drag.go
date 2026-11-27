package scrollbar

import "github.com/egoist/mygo/ui"

type Drag struct {
	Active bool
	Grab   float32
}

func (d *Drag) Press(bar ui.Scrollbar, y float32) (offset float32, jumped bool) {
	start, length := bar.Thumb()
	d.Active = true
	if y >= start && y < start+length {
		d.Grab = y - start
		return 0, false
	}
	d.Grab = length / 2
	return d.Offset(bar, y), true
}

func (d Drag) Offset(bar ui.Scrollbar, y float32) float32 {
	return bar.OffsetAt(y - d.Grab)
}
