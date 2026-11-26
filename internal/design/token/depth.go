package token

import "github.com/egoist/mygo/ui"

type Layer struct{ z int }

func (l Layer) Z() int { return l.z }

var (
	LayerBase       = Layer{}
	LayerSticky     = Layer{10}
	LayerDropTarget = Layer{20}
	LayerPopover    = Layer{100}
	LayerQuickInput = Layer{200}
	LayerToast      = Layer{300}
	LayerTooltip    = Layer{400}
)

type Elevation struct{ y, blur float32 }

var (
	ElevationPopover = Elevation{y: 2, blur: 8}
	ElevationDrag    = Elevation{y: 2, blur: 8}
	ElevationDialog  = Elevation{y: 4, blur: 16}
)

func (e Elevation) Apply(el ui.Element, p *Palette) ui.Element {
	return el.Shadow(0, e.y, e.blur, 0, p.Shadow)
}
