package symbolnav

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/host"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/search/finder"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	peekKey       = "reference-peek"
	peekLabel     = "Peek"
	resultsLabel  = "Peek results"
	previewLabel  = "Peek preview"
	closeLabel    = "Close Peek"
	closeKeys     = "Esc"
	loadingLabel  = "Loading..."
	definitionTag = "Definition"
)

type Env struct {
	Host  host.Host
	Open  func(loc finder.Location, o finder.OpenOptions)
	Focus func()
}

type Anchor struct {
	Area   ui.Rect
	Top    float32
	Bottom float32
}

type row struct {
	file  *finder.FileNode
	match int
}

type Peek struct {
	env      Env
	open     bool
	loading  bool
	since    time.Time
	delay    time.Duration
	kind     Kind
	word     string
	res      Result
	rows     []row
	list     ui.ListState
	selected int
	shown    int
	preview  preview
	focus    bool
}

func New(env Env) *Peek {
	p := &Peek{env: env, shown: -1}
	p.preview.env = env
	p.list.Selected = &p.selected
	p.list.Header = func(i int) bool { return i >= 0 && i < len(p.rows) && p.rows[i].match < 0 }
	p.list.Label = func(i int) string {
		if i < 0 || i >= len(p.rows) || p.rows[i].match < 0 {
			return ""
		}
		return lineText(p.rows[i].file.Matches[p.rows[i].match].Preview)
	}
	return p
}

func (p *Peek) Visible() bool { return p.open }

func (p *Peek) Begin(kind Kind, word string, delay time.Duration) {
	p.Close()
	p.open, p.loading, p.since, p.delay, p.kind, p.word = true, true, time.Now(), delay, kind, word
}

func (p *Peek) Show(res Result, near Near) {
	if !p.open {
		return
	}
	p.loading, p.res = false, res
	p.rows = p.rows[:0]
	for _, f := range res.Files {
		p.rows = append(p.rows, row{file: f, match: -1})
		for i := range f.Matches {
			p.rows = append(p.rows, row{file: f, match: i})
		}
	}
	fi, mi := res.Nearest(near)
	p.selected = p.rowOf(fi, mi)
	p.shown, p.focus = -1, true
	p.list.ScrollIntoView(p.selected)
	p.syncPreview()
}

func (p *Peek) rowOf(file, match int) int {
	for i, r := range p.rows {
		if r.match == match && r.file == p.res.Files[file] {
			return i
		}
	}
	return 0
}

func (p *Peek) Close() {
	p.open, p.loading = false, false
	p.res, p.rows, p.selected, p.shown, p.focus = Result{}, p.rows[:0], 0, -1, false
	p.preview.reset()
}

func (p *Peek) Step(delta int) {
	n := len(p.rows)
	if !p.open || p.loading || n == 0 {
		return
	}
	step := 1
	if delta < 0 {
		step = -1
	}
	for i := 1; i <= n; i++ {
		at := ((p.selected+step*i)%n + n) % n
		if p.rows[at].match >= 0 {
			p.selected = at
			break
		}
	}
	p.list.ScrollIntoView(p.selected)
	p.syncPreview()
}

func (p *Peek) current() (row, bool) {
	if p.selected < 0 || p.selected >= len(p.rows) || p.rows[p.selected].match < 0 {
		return row{}, false
	}
	return p.rows[p.selected], true
}

func (p *Peek) syncPreview() {
	if p.selected == p.shown {
		return
	}
	r, ok := p.current()
	if !ok {
		return
	}
	p.shown = p.selected
	p.preview.show(p.res, r.file, r.match, p.Dismiss)
}

func (p *Peek) Dismiss() {
	p.Close()
	p.env.Focus()
}

func (p *Peek) openCurrent(side bool) {
	r, ok := p.current()
	if !ok {
		return
	}
	loc := p.res.Location(r.file, r.match)
	p.Close()
	p.env.Open(loc, finder.OpenOptions{Preview: true, Focus: true, Side: side})
}

func lineText(preview string) string { return strings.TrimLeft(preview, " \t") }

func whole(v float32) float32 { return float32(math.Round(float64(v))) }

func place(a Anchor, want float32) (y, h float32) {
	areaBottom := a.Area.Y + a.Area.H
	top, bottom := min(max(a.Top, a.Area.Y), areaBottom), min(max(a.Bottom, a.Area.Y), areaBottom)
	below, above := areaBottom-bottom, top-a.Area.Y
	switch {
	case below >= want:
		return whole(bottom), want
	case above >= want:
		return whole(top - want), want
	case max(below, above) < token.PaneMinHeight.Px():
		return whole(a.Area.Y), min(want, a.Area.H)
	case below >= above:
		return whole(bottom), whole(below)
	}
	return whole(a.Area.Y), whole(above)
}

func (p *Peek) View(c *ui.Context, a Anchor) {
	if !p.open {
		return
	}
	if p.loading {
		if wait := p.delay - time.Since(p.since); wait > 0 {
			c.After(wait)
			return
		}
	}
	if a.Area.W <= 0 || a.Area.H <= 0 {
		c.Invalidate()
		return
	}
	p.syncPreview()
	pal := token.Of(c)
	y, h := place(a, token.ReferencePeekHeight.Px())
	var dismissed, outside bool
	ui.Overlay(c, func() {
		panel := ui.Column(c.Key(peekKey)).Absolute().Left(whole(a.Area.X)).Top(y).Width(whole(a.Area.W)).Height(h).
			Background(pal.Layer.Editor).BorderWidth(token.BorderWidth.Px(), 0, token.BorderWidth.Px(), 0).BorderColor(pal.Accent.Focus).
			ZIndex(token.LayerPopover.Z()).Role(ui.RoleDialog).Label(peekLabel)
		token.ElevationPopover.Apply(panel, pal)
		panel.Children(func() {
			p.header(c, pal, &dismissed)
			p.body(c, pal, a.Area.W)
		})
		dismissed = dismissed || panel.OverlayShortcut(0, ui.KeyEscape)
		outside = panel.PressedOutside()
	})
	switch {
	case dismissed:
		p.Dismiss()
	case outside:
		p.Close()
	}
}

func (p *Peek) header(c *ui.Context, pal *token.Palette, dismissed *bool) {
	ui.Row(c).FillWidth().Height(token.HeaderHeight.Px()).Shrink(0).AlignItems(ui.Center).Padding(0, token.PadRow.Px()).
		Gap(token.GapControl.Px()).Background(pal.Layer.SectionHeader).Children(func() {
		ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(token.GapRowIcon.Px()).Children(func() {
			name, dir, meta := loadingLabel, "", ""
			if r, ok := p.current(); ok && !p.loading {
				name, dir = r.file.Name, r.file.Dir
				meta = p.kind.Title() + " (" + strconv.Itoa(p.res.Total) + ")"
			}
			token.TextStrong.Apply(ui.Text(c, name)).TextColor(pal.Text.Primary).SingleLine().Shrink(0)
			if dir != "" {
				token.TextLabel.Apply(ui.Text(c, dir)).TextColor(pal.Text.Secondary).SingleLine().MinWidth(0).Shrink(1)
			}
			if meta != "" {
				token.TextLabel.Apply(ui.Text(c, meta)).TextColor(pal.Text.Muted).SingleLine().Shrink(0)
			}
		})
		if action.IconButton(c, icons.Close, closeLabel, action.IconOptions{Keys: closeKeys}).Clicked() {
			*dismissed = true
		}
	})
}

func listWidth(total float32) float32 {
	width := whole(total * token.PeekListShare / (token.PeekListShare + token.PeekPreviewShare))
	return max(min(width, total-token.PeekPreviewMinWidth.Px()), token.PeekListMinWidth.Px())
}

func (p *Peek) body(c *ui.Context, pal *token.Palette, width float32) {
	if p.loading {
		feedback.CodeSkeleton(c, loadingLabel)
		return
	}
	showList := p.res.Total > 1
	ui.Row(c).Grow(1).FillWidth().MinHeight(0).AlignItems(ui.Stretch).Children(func() {
		pane := ui.Column(c.Key("peek-preview-pane")).Grow(1).MinWidth(token.PeekPreviewMinWidth.Px()).MinHeight(0).FillHeight()
		pane.Children(func() { p.previewPane(c) })
		if !showList {
			return
		}
		ui.Column(c.Key("peek-results-pane")).Width(listWidth(width)).Shrink(0).MinHeight(0).FillHeight().
			Background(pal.Layer.SideBar).BorderWidth(0, 0, 0, token.SeparatorWidth.Px()).BorderColor(pal.Border.Section).Children(func() { p.results(c, pal) })
	})
	if p.focus {
		p.focus = false
		if showList {
			p.list.Focus(c)
		} else {
			p.preview.handle.Focus(c)
		}
	}
}

func (p *Peek) previewPane(c *ui.Context) {
	pv := &p.preview
	switch {
	case pv.loading:
		feedback.CodeSkeleton(c, loadingLabel)
	case pv.st == nil:
		feedback.Empty(c, feedback.EmptyOptions{Title: pv.message})
	default:
		editor.View(c.Key("peek-preview"), pv.st).Grow(1).FillWidth().MinHeight(0).Label(previewLabel).Bind(&pv.handle)
	}
}

func (p *Peek) results(c *ui.Context, pal *token.Palette) {
	l := list.View(c, &p.list, len(p.rows), list.ViewOptions{Label: resultsLabel, Tree: true}, func(i int) list.Item { return p.item(pal, i) })
	if l.Submitted() {
		p.openCurrent(false)
	} else if p.list.Shortcut(c, ui.Cmd, ui.KeyEnter) {
		p.openCurrent(true)
	}
	p.syncPreview()
}

func (p *Peek) item(pal *token.Palette, i int) list.Item {
	r := p.rows[i]
	if r.match < 0 {
		return finder.FileItem(r.file)
	}
	it := finder.MatchItem(pal, r.file.Matches[r.match])
	if p.res.Definition(r.file, r.match) {
		it.Aside = definitionTag
	}
	return it
}
