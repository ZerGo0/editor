package gitui

import (
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/git/diffmode"
	"github.com/ZerGo0/editor/internal/multidiff"
	"github.com/egoist/mygo/ui/editor"
)

const (
	commitStateKey = "gitui.commit"
	layoutStateKey = "gitui.layout"
	logFilterKey   = "gitui.log"
	viewedKey      = "gitui.viewed"
	diffPrefsKey   = "gitui.diff"
)

type savedCommit struct {
	Message string   `json:"message,omitempty"`
	History []string `json:"history,omitempty"`
}

type savedLogFilter struct {
	Text   string `json:"text,omitempty"`
	Branch string `json:"branch,omitempty"`
	Author string `json:"author,omitempty"`
	Path   string `json:"path,omitempty"`
}

type viewLayout struct {
	Collapsed   map[string]bool    `json:"collapsed,omitempty"`
	Weights     map[string]float32 `json:"weights,omitempty"`
	LogSplit    float32            `json:"logSplit,omitempty"`
	FilesSplit  float32            `json:"filesSplit,omitempty"`
	DiffSplit   float32            `json:"diffSplit,omitempty"`
	TreeWidth   float32            `json:"treeWidth,omitempty"`
	TreeHidden  bool               `json:"treeHidden,omitempty"`
	MergeSplit  float32            `json:"mergeSplit,omitempty"`
	ResultSplit float32            `json:"resultSplit,omitempty"`
	Columns     logWidths          `json:"columns"`
}

type diffPrefs struct {
	Layout      *int              `json:"layout,omitempty"`
	Inline      editor.DiffInline `json:"inline,omitempty"`
	ExpandAll   bool              `json:"expandAll,omitempty"`
	HideNumbers bool              `json:"hideNumbers,omitempty"`
	Whitespace  string            `json:"whitespace,omitempty"`
	ShowMoves   *bool             `json:"showMoves,omitempty"`
}

type diffChoice struct {
	layout int
	mode   diffmode.Options
}

func (g *Git) restoreState() {
	project := g.h.ProjectState()
	var cm savedCommit
	if project.Load(commitStateKey, &cm) {
		g.commit.message = cm.Message
		g.commit.history = cm.History[:min(len(cm.History), messageHistoryLimit)]
	}
	var f savedLogFilter
	if project.Load(logFilterKey, &f) {
		if f.Branch == "" {
			f.Branch = g.log.defaultBranch()
		}
		g.log.filter = logFilter{text: f.Text, branch: f.Branch, author: f.Author, path: f.Path}
		g.log.applied = g.log.filter
	}
	var marks []string
	project.Load(viewedKey, &marks)
	g.viewed = multidiff.NewMemory(marks...)
	g.restoreLayout()
	g.loadDiffPrefs()
	project.Track(commitStateKey, func() any { return savedCommit{Message: g.commit.message, History: g.commit.history} })
	project.Track(logFilterKey, func() any {
		f := g.log.filter
		return savedLogFilter{Text: f.text, Branch: f.branch, Author: f.author, Path: f.path}
	})
	project.Track(viewedKey, func() any { return g.viewed.Marks() })
	project.Track(layoutStateKey, g.saveLayout)
}

func (g *Git) restoreLayout() {
	var l viewLayout
	if g.h.ProjectState().Load(layoutStateKey, &l) {
		if l.Weights != nil {
			g.changes.stack.Weights = l.Weights
		}
		g.changes.stack.Collapsed = l.Collapsed
		if l.LogSplit > 0 {
			g.log.split = l.LogSplit
		}
		g.log.details.split = l.FilesSplit
	}
	if l.MergeSplit <= 0 {
		l.MergeSplit = mergeSplitDefault
	}
	if l.ResultSplit <= 0 {
		l.ResultSplit = resultSplitDefault
	}
	l.Columns = l.Columns.clamped()
	l.Collapsed, l.Weights = nil, nil
	g.layout = l
}

func (g *Git) saveLayout() any {
	l := g.layout
	l.Collapsed, l.Weights = g.changes.stack.Collapsed, g.changes.stack.Weights
	l.LogSplit, l.FilesSplit = g.log.split, g.log.details.split
	return l
}

func (g *Git) loadDiffPrefs() {
	var p diffPrefs
	if !g.h.GlobalState().Load(diffPrefsKey, &p) {
		return
	}
	o := &g.diffOpts
	if p.Layout != nil && *p.Layout >= 0 && *p.Layout < len(layoutNames) {
		o.layout, o.layoutSet = *p.Layout, true
	}
	o.inline, o.expandAll, o.hideNumbers = p.Inline, p.ExpandAll, p.HideNumbers
	if mode, ok := git.ParseWhitespaceMode(p.Whitespace); ok && p.Whitespace != "" {
		diffmode.Session.Whitespace, o.whitespaceSet = mode, true
	}
	if p.ShowMoves != nil {
		diffmode.Session.ShowMoves, o.movesSet = *p.ShowMoves, true
	}
}

func (g *Git) saveDiffPrefs() {
	o := g.diffOpts
	p := diffPrefs{Inline: o.inline, ExpandAll: o.expandAll, HideNumbers: o.hideNumbers}
	if o.layoutSet {
		p.Layout = &o.layout
	}
	if o.whitespaceSet {
		p.Whitespace = diffmode.Session.Whitespace.Key()
	}
	if o.movesSet {
		moves := diffmode.Session.ShowMoves
		p.ShowMoves = &moves
	}
	g.h.GlobalState().Set(diffPrefsKey, p)
}

func currentDiffChoice(layout int) diffChoice {
	return diffChoice{layout: layout, mode: diffmode.Session}
}

func (g *Git) noteDiffChoice(before diffChoice, layout int) {
	after := currentDiffChoice(layout)
	if after == before {
		return
	}
	o := &g.diffOpts
	if after.layout != before.layout {
		o.layout, o.layoutSet = after.layout, true
	}
	o.whitespaceSet = o.whitespaceSet || after.mode.Whitespace != before.mode.Whitespace
	o.movesSet = o.movesSet || after.mode.ShowMoves != before.mode.ShowMoves
	g.saveDiffPrefs()
}

func (g *Git) newDiffView() *multidiff.View {
	v := multidiff.New()
	v.SetLayout(g.diffOpts.layout)
	v.SetTree(g.layout.TreeWidth, !g.layout.TreeHidden)
	return v
}

func (g *Git) noteTree(v *multidiff.View, width float32, shown bool) {
	if w, s := v.Tree(); w != width || s != shown {
		g.layout.TreeWidth, g.layout.TreeHidden = w, !s
	}
}
