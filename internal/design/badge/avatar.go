package badge

import (
	"strings"
	"unicode"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	initialsScale = 0.42
	maxInitials   = 2
)

func Initials(name string) string {
	var out []rune
	for _, word := range strings.FieldsFunc(name, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		out = append(out, unicode.ToUpper([]rune(word)[0]))
		if len(out) == maxInitials {
			break
		}
	}
	return string(out)
}

func Avatar(c *ui.Context, name string, size token.Size) ui.Element {
	p := token.Of(c)
	px := size.Px()
	if px <= 0 {
		px = token.AvatarSize.Px()
	}
	return ui.Box(c).Size(px, px).Radius(token.RadiusPill.Px()).Background(p.AvatarColor(name)).Center().Shrink(0).
		Role(ui.RoleImage).Label(name).Children(func() {
		ui.Text(c, Initials(name)).FontSize(px * initialsScale).FontWeight(token.WeightSemibold).
			TextColor(p.OnAvatar).SingleLine().AccessHidden(true)
	})
}
