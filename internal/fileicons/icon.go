package fileicons

import (
	"sync"

	"github.com/egoist/mygo/ui"
)

const DefaultSize = 16

type lazySVG struct {
	once sync.Once
	svg  *ui.SVG
}

type Icon struct {
	name      string
	darkPath  string
	lightPath string
	dark      lazySVG
	light     lazySVG
}

func (i *Icon) Name() string {
	if i == nil {
		return ""
	}
	return i.name
}

func (i *Icon) View(c *ui.Context) ui.Element {
	return i.ViewSize(c, DefaultSize)
}

func (i *Icon) ViewSize(c *ui.Context, size float64) ui.Element {
	side := float32(size)
	svg := i.svg(c.Theme().Dark)
	if svg == nil {
		return ui.Box(c).Size(side, side).Shrink(0)
	}
	return ui.Image(c, svg).Size(side, side).Shrink(0)
}

func (i *Icon) variant(dark bool) (*lazySVG, string) {
	if !dark && i.lightPath != "" {
		return &i.light, i.lightPath
	}
	return &i.dark, i.darkPath
}

func (i *Icon) svg(dark bool) *ui.SVG {
	if i == nil {
		return nil
	}
	slot, path := i.variant(dark)
	slot.once.Do(func() {
		data, err := svgFiles.ReadFile(svgDir + path)
		if err != nil {
			return
		}
		slot.svg, _ = ui.ParseSVG(data)
	})
	return slot.svg
}
