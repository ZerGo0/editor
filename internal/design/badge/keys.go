package badge

import (
	"strings"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const modifierGlyphs = "⌘⇧⌥⌃"

func KeyCaps(keys string) [][]string {
	var chords [][]string
	for _, chord := range strings.Fields(keys) {
		var caps []string
		rest := []rune(chord)
		for len(rest) > 1 && strings.ContainsRune(modifierGlyphs, rest[0]) {
			caps = append(caps, string(rest[0]))
			rest = rest[1:]
		}
		caps = append(caps, string(rest))
		chords = append(chords, caps)
	}
	return chords
}

func Keys(c *ui.Context, keys string) ui.Element {
	p := token.Of(c)
	return ui.Row(c).Gap(token.GapInline.Px()).AlignItems(ui.Center).Shrink(0).Label(keys).Children(func() {
		for _, chord := range KeyCaps(keys) {
			ui.Row(c).Gap(token.GapTight.Px()).AlignItems(ui.Center).AccessHidden(true).Children(func() {
				for _, k := range chord {
					ui.Row(c).Height(token.KeyCapHeight.Px()).MinWidth(token.KeyCapMinWidth.Px()).Padding(0, token.PadKeyX.Px()).Radius(token.RadiusControl.Px()).
						AlignItems(ui.Center).Justify(ui.Center).Background(p.Key.Background).Border(token.BorderWidth.Px(), p.Key.Border).
						BorderWidth(token.BorderWidth.Px(), token.BorderWidth.Px(), token.KeyCapBottomWidth.Px(), token.BorderWidth.Px()).Children(func() {
						token.TextCaption.WholeText(c, k).TextColor(p.Key.Text).SingleLine()
					})
				}
			})
		}
	})
}
