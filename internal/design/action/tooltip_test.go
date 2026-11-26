package action

import (
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

func TestTooltipBelowTitleBarOpensBelow(t *testing.T) {
	const (
		tipHeight = 400
		lowY      = 300
		restTime  = time.Second
	)
	for name, top := range map[string]float32{"under title bar": token.TitleBarHeight.Px() + token.GapInline.Px(), "low": lowY} {
		t.Run(name, func(t *testing.T) {
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Padding(top, 0, 0, 0).Children(func() {
					IconButton(c, icons.Refresh, "Refresh", IconOptions{})
				})
			}, testWidth, tipHeight)
			b, ok := tt.AccessByRole(ui.RoleButton, "Refresh")
			if !ok {
				t.Fatal("no button")
			}
			tt.Move(b.Bounds.X+b.Bounds.W/2, b.Bounds.Y+b.Bounds.H/2)
			tt.Advance(restTime)
			tt.Frame()
			var tip *ui.AccessInfo
			for _, a := range tt.AccessAll() {
				if a.Role == ui.RoleTooltip {
					tip = &a
				}
			}
			if tip == nil {
				t.Fatal("no tooltip shows")
			}
			below := tip.Bounds.Y >= b.Bounds.Y+b.Bounds.H
			if want := top < lowY; below != want {
				t.Errorf("tooltip at y %v for a button at y %v: below = %v, want %v", tip.Bounds.Y, b.Bounds.Y, below, want)
			}
			if below && tip.Bounds.Y < token.TitleBarHeight.Px() {
				t.Errorf("tooltip at y %v overlaps the title bar", tip.Bounds.Y)
			}
		})
	}
}
