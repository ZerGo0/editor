package workspace

import (
	"log"
	"math"
	"slices"
	"sort"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/bracketcolor"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	bracketMargin   = 200
	bracketMaxBytes = 2 << 20
)

type bracketState struct {
	scans map[string]*bracketScan
	views map[*editor.State]bracketView
}

type bracketScan struct {
	version  uint64
	lang     *editor.Language
	running  bool
	brackets []bracketcolor.Bracket
}

type bracketView struct {
	version  uint64
	lang     *editor.Language
	from, to int
	dark     bool
}

func (w *Workspace) initBrackets() {
	w.brackets = bracketState{scans: map[string]*bracketScan{}, views: map[*editor.State]bracketView{}}
}

func bracketPalette(p *token.Palette) []ui.Color {
	return []ui.Color{p.Severity.Warning, p.Accent.Base, p.Severity.Success}
}

func (w *Workspace) colorBrackets(c *ui.Context, tab *layout.Tab) {
	st := tab.Editor
	buf := st.Buffer
	lang := buf.Language()
	if !w.settings.Bool(settings.BracketColors) || !bracketcolor.Supports(lang) || buf.Len() > bracketMaxBytes {
		if _, ok := w.brackets.views[st]; ok {
			st.SetReplacements(nil)
			delete(w.brackets.views, st)
		}
		return
	}
	scan := w.brackets.scans[tab.Path]
	if scan == nil {
		scan = &bracketScan{}
		w.brackets.scans[tab.Path] = scan
	}
	version := buf.Version()
	if scan.version != version || scan.lang != lang || scan.brackets == nil {
		w.startBracketScan(scan, buf, lang)
		if scan.version != version || scan.lang != lang {
			return
		}
	}
	first, last := st.VisibleLines()
	t := token.Of(c)
	v, ok := w.brackets.views[st]
	if ok && v.version == version && v.lang == lang && v.dark == t.Dark && first >= v.from && last <= v.to {
		return
	}
	v = bracketView{version: version, lang: lang, dark: t.Dark, from: max(0, first-bracketMargin), to: last + bracketMargin}
	w.brackets.views[st] = v
	start := buf.Offset(editor.Pos{Line: v.from})
	end := math.MaxInt
	if v.to+1 < buf.LineCount() {
		end = buf.Offset(editor.Pos{Line: v.to + 1})
	}
	colors := bracketPalette(t)
	bs := scan.brackets
	i := sort.Search(len(bs), func(i int) bool { return bs[i].Offset >= start })
	var rs []editor.Replacement
	for ; i < len(bs) && bs[i].Offset < end; i++ {
		b := bs[i]
		color := t.Severity.Error
		if !b.Unexpected {
			color = colors[b.Depth%len(colors)]
		}
		rs = append(rs, editor.Replacement{Start: b.Offset, End: b.Offset + 1, Text: string(b.Char), Color: color})
	}
	st.SetReplacements(rs)
}

func (w *Workspace) startBracketScan(scan *bracketScan, buf *editor.Buffer, lang *editor.Language) {
	if scan.running {
		return
	}
	snap := buf.Snapshot()
	if w.win == nil {
		bs, err := bracketcolor.Scan(w.ctx, snap, lang)
		w.bracketsScanned(scan, snap.Version(), lang, bs, err)
		return
	}
	scan.running = true
	applog.Go(func() {
		bs, err := bracketcolor.Scan(w.ctx, snap, lang)
		w.updateQuietly(func() bool {
			scan.running = false
			return w.bracketsScanned(scan, snap.Version(), lang, bs, err)
		})
	})
}

func (w *Workspace) bracketsScanned(scan *bracketScan, version uint64, lang *editor.Language, bs []bracketcolor.Bracket, err error) bool {
	if err != nil {
		if w.ctx.Err() == nil {
			log.Printf("bracket colors: %v", err)
		}
		return false
	}
	if bs == nil {
		bs = []bracketcolor.Bracket{}
	}
	changed := !slices.Equal(scan.brackets, bs)
	scan.version, scan.lang, scan.brackets = version, lang, bs
	return changed
}

func (w *Workspace) dropUnusedBrackets(open map[string]bool) {
	for p := range w.brackets.scans {
		if !open[p] {
			delete(w.brackets.scans, p)
		}
	}
	editors := map[*editor.State]bool{}
	for _, t := range w.tabs() {
		editors[t.Editor] = true
	}
	for st := range w.brackets.views {
		if !editors[st] {
			delete(w.brackets.views, st)
		}
	}
}
