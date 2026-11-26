package gitui

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
)

const (
	maxLanes      = 16
	curveFraction = 0.5
	half          = 2
)

func graphWidth(rows []git.GraphRow) float32 {
	lanes := 1
	for _, r := range rows {
		lanes = max(lanes, r.Width)
	}
	return float32(min(lanes, maxLanes))*token.GraphLaneWidth.Px() + token.Space6.Px()
}

func laneX(r ui.Rect, lane int) float32 {
	return r.X + token.Space6.Px()/half + float32(lane)*token.GraphLaneWidth.Px() + token.GraphLaneWidth.Px()/half
}

func visibleLane(lane int) bool {
	return lane < maxLanes
}

func drawGraphRow(p *ui.Painter, r ui.Rect, pal *token.Palette, bg ui.Color, row git.GraphRow, merge bool) {
	top, mid, bottom := r.Y, r.Y+r.H/half, r.Y+r.H
	stroke := token.GraphLineWidth.Px()
	for _, k := range row.Through {
		if visibleLane(k) {
			x := laneX(r, k)
			p.Line(x, top, x, bottom, stroke, pal.GraphLane(k))
		}
	}
	cx := laneX(r, min(row.Column, maxLanes-1))
	for _, k := range row.Incoming {
		if visibleLane(k) {
			p.StrokePath(curve(laneX(r, k), top, cx, mid), stroke, pal.GraphLane(k))
		}
	}
	for _, e := range row.Edges {
		if visibleLane(e.Column) {
			p.StrokePath(curve(cx, mid, laneX(r, e.Column), bottom), stroke, pal.GraphLane(e.Column))
		}
	}
	color := pal.GraphLane(row.Column)
	dot := (&ui.Path{}).Circle(cx, mid, token.GraphDotSize.Px()/half)
	if merge {
		p.FillPath(dot, bg)
		p.StrokePath(dot, stroke, color)
		return
	}
	p.FillPath(dot, color)
}

func drawGraphContinue(p *ui.Painter, r ui.Rect, pal *token.Palette, row git.GraphRow) {
	stroke := token.GraphLineWidth.Px()
	lanes := append([]int{}, row.Through...)
	for _, e := range row.Edges {
		lanes = append(lanes, e.Column)
	}
	for _, k := range lanes {
		if visibleLane(k) {
			x := laneX(r, k)
			p.Line(x, r.Y, x, r.Y+r.H, stroke, pal.GraphLane(k))
		}
	}
}

func curve(x0, y0, x1, y1 float32) *ui.Path {
	path := (&ui.Path{}).MoveTo(x0, y0)
	if x0 == x1 {
		return path.LineTo(x1, y1)
	}
	dy := (y1 - y0) * curveFraction
	return path.CubeTo(x0, y0+dy, x1, y1-dy, x1, y1)
}
