package icons

//go:generate go run ./gen

import (
	"embed"

	"github.com/egoist/mygo/ui"
)

//go:embed *.svg
var files embed.FS

func load(name string) *ui.SVG {
	data, err := files.ReadFile(name + ".svg")
	if err != nil {
		panic(err)
	}
	return ui.MustParseSVG(data)
}
