package token

import (
	"sync"

	"github.com/egoist/mygo/ui"
)

const GraphLaneCount = 5

var (
	GraphLaneWidth = Size{12}
	GraphDotSize   = Size{7}
	GraphLineWidth = Stroke{1.5}
)

var graphLaneHex = [GraphLaneCount]string{"#FFB000", "#DC267F", "#994F00", "#40A0FF", "#B66DFF"}

var graphLanes = struct {
	sync.Mutex
	byPalette map[*Palette][GraphLaneCount]ui.Color
}{byPalette: map[*Palette][GraphLaneCount]ui.Color{}}

func (p *Palette) GraphLane(i int) ui.Color {
	graphLanes.Lock()
	defer graphLanes.Unlock()
	lanes, ok := graphLanes.byPalette[p]
	if !ok {
		for k, hex := range graphLaneHex {
			lanes[k] = ensure(ui.Hex(hex), ContrastLarge, p.Layer.SideBar, p.Layer.Panel, p.Layer.Editor)
		}
		graphLanes.byPalette[p] = lanes
	}
	return lanes[(i%GraphLaneCount+GraphLaneCount)%GraphLaneCount]
}
