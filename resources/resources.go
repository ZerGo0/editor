package resources

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sync"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	fontDir = "fonts"
	fontExt = ".ttf"
)

//go:embed icon.svg
var iconSVG []byte

//go:embed fonts/*.ttf
var fonts embed.FS

var (
	logoOnce sync.Once
	logo     *ui.SVG
	fontOnce sync.Once
	fontErr  error
)

func Logo() *ui.SVG {
	logoOnce.Do(func() {
		svg, err := ui.ParseSVG(iconSVG)
		if err == nil {
			logo = svg
		}
	})
	return logo
}

func RegisterFonts() error {
	fontOnce.Do(func() { fontErr = registerFonts(fonts) })
	return fontErr
}

func registerFonts(fsys fs.FS) error {
	entries, err := fs.ReadDir(fsys, fontDir)
	if err != nil {
		return fmt.Errorf("read bundled fonts: %w", err)
	}
	for _, e := range entries {
		if path.Ext(e.Name()) != fontExt {
			continue
		}
		data, err := fs.ReadFile(fsys, path.Join(fontDir, e.Name()))
		if err != nil {
			return fmt.Errorf("read font %s: %w", e.Name(), err)
		}
		if err := ui.RegisterFont(data, token.FontEditor); err != nil {
			return fmt.Errorf("register font %s: %w", e.Name(), err)
		}
	}
	return nil
}
