package gitui

import (
	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
)

const (
	laneWidth     = 12
	maxLanes      = 16
	dotRadius     = 3.5
	lineWidth     = 1.5
	graphPadding  = 6
	curveFraction = 0.5
)

func graphWidth(rows []git.GraphRow) float32 {
	lanes := 1
	for _, r := range rows {
		lanes = max(lanes, r.Width)
	}
	return float32(min(lanes, maxLanes))*laneWidth + graphPadding
}

func laneColor(t *ui.Theme, lane int) ui.Color {
	colors := []ui.Color{t.Accent, t.Success, t.Warning, t.Danger, t.TextMuted}
	return colors[lane%len(colors)]
}

func laneX(r ui.Rect, lane int) float32 {
	return r.X + graphPadding/2 + float32(lane)*laneWidth + laneWidth/2
}

func drawGraphRow(p *ui.Painter, r ui.Rect, t *ui.Theme, row git.GraphRow, merge bool) {
	top, mid, bottom := r.Y, r.Y+r.H/2, r.Y+r.H
	visible := func(lane int) bool { return lane < maxLanes }
	for _, k := range row.Through {
		if visible(k) {
			x := laneX(r, k)
			p.Line(x, top, x, bottom, lineWidth, laneColor(t, k))
		}
	}
	cx := laneX(r, min(row.Column, maxLanes-1))
	for _, k := range row.Incoming {
		if visible(k) {
			p.StrokePath(curve(laneX(r, k), top, cx, mid), lineWidth, laneColor(t, k))
		}
	}
	for _, e := range row.Edges {
		if visible(e.Column) {
			p.StrokePath(curve(cx, mid, laneX(r, e.Column), bottom), lineWidth, laneColor(t, e.Column))
		}
	}
	color := laneColor(t, row.Column)
	dot := (&ui.Path{}).Circle(cx, mid, dotRadius)
	if merge {
		p.FillPath(dot, t.Background)
		p.StrokePath(dot, lineWidth, color)
		return
	}
	p.FillPath(dot, color)
}

func curve(x0, y0, x1, y1 float32) *ui.Path {
	path := (&ui.Path{}).MoveTo(x0, y0)
	if x0 == x1 {
		return path.LineTo(x1, y1)
	}
	dy := (y1 - y0) * curveFraction
	return path.CubeTo(x0, y0+dy, x1, y1-dy, x1, y1)
}
