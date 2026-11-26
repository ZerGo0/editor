package workspace

import (
	"strings"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/ZerGo0/editor/internal/viewer"
	"github.com/egoist/mygo/ui"
)

const (
	fontListSeparator = ","
	fontQuotes        = "'\""
)

type fontChoice struct {
	list   string
	family string
}

func fontFamilies(list string) []string {
	var out []string
	for _, f := range strings.Split(list, fontListSeparator) {
		if f = strings.TrimSpace(strings.Trim(strings.TrimSpace(f), fontQuotes)); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func pickFontFamily(list string, installed func() map[string]string) string {
	families := fontFamilies(list)
	if len(families) == 0 {
		return token.FontEditor
	}
	names := installed()
	for _, f := range families {
		if name, ok := names[strings.ToLower(f)]; ok {
			return name
		}
	}
	return token.FontEditor
}

func installedFonts() map[string]string {
	families := ui.FontFamilies()
	names := make(map[string]string, len(families))
	for _, f := range families {
		names[strings.ToLower(f.Name)] = f.Name
	}
	return names
}

func (w *Workspace) editorFont() ui.Font {
	list := w.settings.String(settings.FontFamily)
	if w.font.family == "" || w.font.list != list {
		w.font = fontChoice{list: list, family: pickFontFamily(list, installedFonts)}
	}
	return ui.Font{Family: w.font.family, Size: float32(w.settings.Int(settings.FontSize))}
}

func (w *Workspace) viewerOptions() viewer.Options {
	f := w.editorFont()
	return viewer.Options{FontFamily: f.Family, FontSize: f.Size, PreviewSize: float32(w.settings.Int(settings.MarkdownFontSize))}
}
