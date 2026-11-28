package gitui

import (
	"math"

	"github.com/ZerGo0/editor/internal/design/pane"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type logColumn int

const (
	colAuthor logColumn = iota
	colDate
	colHash
	logColumnCount
)

const (
	subjectColumnTitle = "Subject"
	maxColumnFactor    = 4
)

var logColumnTitles = [logColumnCount]string{"Author", "Date", "Hash"}

type logWidths [logColumnCount]float32

func defaultColumnWidth(col logColumn) float32 {
	switch col {
	case colAuthor:
		return authorWidth()
	case colDate:
		return token.ColumnMedium.Px()
	}
	return token.ColumnSmall.Px()
}

func minColumnWidth() float32 { return token.ColumnSmall.Px() / half }

func maxColumnWidth() float32 { return maxColumnFactor * token.ColumnMedium.Px() }

func (w logWidths) of(col logColumn) float32 {
	if w[col] > 0 {
		return w[col]
	}
	return defaultColumnWidth(col)
}

func (w logWidths) clamped() logWidths {
	for col := range logColumnCount {
		if w[col] > 0 {
			w[col] = min(max(w[col], minColumnWidth()), maxColumnWidth())
		}
	}
	return w
}

func (s *logState) columnHeader(c *ui.Context, gw float32) {
	if s.g == nil {
		return
	}
	cols := s.columns(gw, 0)
	if !cols.author && !cols.date && !cols.hash {
		return
	}
	p := token.Of(c)
	room := s.rowRoom(gw)
	end := token.PadRow.Px() + room - float32(math.Floor(float64(room)))
	ui.Row(c.Key("log-columns")).FillWidth().Height(token.HeaderHeight.Px()).Shrink(0).Padding(token.Space0.Px(), end, token.Space0.Px(), token.PadRow.Px()).
		Gap(token.GapInline.Px()).AlignItems(ui.Center).BorderWidth(0, 0, token.BorderWidth.Px(), 0).BorderColor(p.Border.Default).
		Role(ui.RoleGroup).Label("Log Columns").Children(func() {
		columnTitle(c, subjectColumnTitle).Grow(1).Shrink(1).MinWidth(0)
		for col := range logColumnCount {
			if *cols.shown(col) {
				s.columnCell(c, col, cols.slack)
			}
		}
	})
}

func columnTitle(c *ui.Context, title string) ui.Element {
	return token.TextCaption.Apply(ui.Text(c, title)).FontWeight(token.WeightSemibold).TextColor(token.Of(c).Text.Secondary).SingleLine()
}

func (s *logState) columnCell(c *ui.Context, col logColumn, slack float32) {
	widths := &s.g.layout.Columns
	w := widths.of(col)
	title := logColumnTitles[col]
	offset := -token.SashSize.Px() / half
	ui.Row(c.Key(col)).Width(w).FillHeight().Shrink(0).AlignItems(ui.Center).Children(func() {
		columnTitle(c, title).Shrink(1).MinWidth(0)
		sash, r := pane.Sash(c.Key("sash"), pane.SashOptions{Label: "Resize " + title + " Column"})
		sash.Top(0).Bottom(0).Left(offset)
		switch {
		case r.Reset:
			widths[col] = 0
			c.Invalidate()
		case r.Delta != 0:
			widths[col] = min(max(w-r.Delta, minColumnWidth()), maxColumnWidth(), w+slack)
			c.Invalidate()
		}
	})
}
