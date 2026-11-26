package prefs

import (
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

func toolbar(c *ui.Context, label string, fn func()) {
	p := token.Of(c)
	ui.Row(c).Gap(token.GapControl.Px()).Padding(token.GapStack.Px(), token.PadView.Px()).AlignItems(ui.Center).FillWidth().Shrink(0).
		BorderWidth(0, 0, token.BorderWidth.Px(), 0).BorderColor(p.Border.Section).Role(ui.RoleToolbar).Label(label).Children(fn)
}

func problems(c *ui.Context, errs []string) {
	if len(errs) == 0 {
		return
	}
	p := token.Of(c)
	ui.Column(c).Gap(token.GapInline.Px()).Padding(token.GapStack.Px(), token.PadView.Px()).FillWidth().Shrink(0).
		Background(p.Severity.WarningBackground).BorderWidth(0, 0, token.BorderWidth.Px(), 0).BorderColor(p.Severity.WarningBorder).
		Role(ui.RoleAlert).Children(func() {
		for _, e := range errs {
			design.Inline(c, token.GapRowIcon, func() {
				ui.Icon(c, feedback.ToneIcon(token.ToneWarning)).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(p.Severity.Warning).Shrink(0)
				design.Text(c, e, token.TextBody).Wrap().Shrink(1).MinWidth(0)
			})
		}
	})
}

func nonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}
