package workspace

import (
	"strings"

	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/insights"
	"github.com/ZerGo0/editor/internal/problems"
	"github.com/ZerGo0/editor/internal/todo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	ProblemsPanel       = "problems"
	TODOPanel           = "todo"
	problemsFilterWidth = 220
)

type problemEntry struct {
	version uint64
	running bool
	items   []problems.Problem
}

type insightState struct {
	todos        *todo.Index
	todoList     insights.List
	problemList  insights.List
	problems     map[string]*problemEntry
	problemsGen  uint64
	configErrors string
}

func (w *Workspace) initInsights() {
	w.insights.todos = todo.NewIndex()
	w.insights.problems = map[string]*problemEntry{}
	w.addPanel(toolView{id: ProblemsPanel, title: "Problems", command: "workbench.actions.view.problems", icon: icons.Problems, view: w.problemsView, badge: w.problemCount, actions: w.problemsActions})
	w.addPanel(toolView{id: TODOPanel, title: "TODO", command: "todo.show", icon: icons.Todo, view: w.todoView, actions: w.todoActions})
}

func (w *Workspace) todoView(c *ui.Context) {
	x := w.insights.todos
	w.insights.todoList.View(c, "TODO", "No TODO comments in this project.", x.Generation(), func() []insights.Group {
		files := x.Files()
		groups := make([]insights.Group, len(files))
		for i, f := range files {
			groups[i].Path = f.Path
			for _, it := range f.Items {
				groups[i].Entries = append(groups[i].Entries, insights.Entry{Icon: icons.Todo, Color: accent, Title: it.Tag, Detail: it.Text, Line: it.Line, Col: it.Col})
			}
		}
		return groups
	}, w.rel, w.OpenAt)
}

func (w *Workspace) problemsActions(c *ui.Context) {
	t := c.Theme()
	ui.TextInput(c, &w.insights.problemList.Filter).Width(problemsFilterWidth).
		Placeholder("Filter (text, path)").Label("Filter Problems").Background(t.Surface)
	iconButton(c, "collapse-problems", "Collapse All", "Collapse All", icons.CollapseAll, false, w.insights.problemList.CollapseAll)
}

func (w *Workspace) todoActions(c *ui.Context) {
	iconButton(c, "collapse-todo", "Collapse All", "Collapse All", icons.CollapseAll, false, w.insights.todoList.CollapseAll)
}

func accent(t *ui.Theme) ui.Color  { return t.Accent }
func danger(t *ui.Theme) ui.Color  { return t.Danger }
func warning(t *ui.Theme) ui.Color { return t.Warning }

func (w *Workspace) problemCount() int {
	n := 0
	for _, e := range w.insights.problems {
		n += len(e.items)
	}
	return n + len(w.configProblems())
}

func (w *Workspace) configProblems() []insights.Group {
	var out []insights.Group
	add := func(path string, errs []string) {
		if len(errs) == 0 {
			return
		}
		g := insights.Group{Path: path}
		for _, e := range errs {
			g.Entries = append(g.Entries, insights.Entry{Icon: icons.Error, Color: danger, Title: e})
		}
		out = append(out, g)
	}
	add(w.env.Settings.Path, w.env.Settings.Errors)
	add(w.settings.Path, w.settings.Errors)
	add(w.env.Keys.Path, w.env.Keys.Current.Errors)
	return out
}

func (w *Workspace) problemsView(c *ui.Context) {
	config := w.configProblems()
	key := w.insights.problemsGen
	var sig strings.Builder
	for _, g := range config {
		for _, e := range g.Entries {
			sig.WriteString(e.Title)
		}
	}
	if s := sig.String(); s != w.insights.configErrors {
		w.insights.configErrors = s
		w.insights.problemsGen++
		key = w.insights.problemsGen
	}
	w.insights.problemList.View(c, "Problems", "No problems in open files.", key, func() []insights.Group {
		groups := config
		for _, t := range w.tabs() {
			e := w.insights.problems[t.Path]
			if e == nil || len(e.items) == 0 || containsGroup(groups, t.Path) {
				continue
			}
			g := insights.Group{Path: t.Path}
			for _, p := range e.items {
				g.Entries = append(g.Entries, problemEntryOf(p))
			}
			groups = append(groups, g)
		}
		return groups
	}, w.rel, w.OpenAt)
}

func containsGroup(groups []insights.Group, path string) bool {
	for _, g := range groups {
		if g.Path == path {
			return true
		}
	}
	return false
}

func problemEntryOf(p problems.Problem) insights.Entry {
	e := insights.Entry{Title: p.Message, Detail: p.Source, Line: p.Line, Col: p.Col}
	switch p.Severity {
	case problems.Error:
		e.Icon, e.Color = icons.Error, danger
	case problems.Warning:
		e.Icon, e.Color = icons.Warning, warning
	default:
		e.Icon, e.Color = icons.Info, accent
	}
	return e
}

func (w *Workspace) updateProblems() {
	for path, d := range w.docs {
		if d.doc == nil || d.doc.Buffer == nil {
			continue
		}
		e := w.insights.problems[path]
		if e == nil {
			e = &problemEntry{}
			w.insights.problems[path] = e
		}
		buf := d.doc.Buffer
		version := buf.Version()
		if e.running || e.version == version && e.items != nil {
			continue
		}
		if buf.Len() > problems.MaxTextSize {
			e.version, e.items = version, []problems.Problem{}
			continue
		}
		e.running = true
		lang, text := buf.Language(), buf.String()
		go func() {
			items := problems.Check(lang, path, text)
			if items == nil {
				items = []problems.Problem{}
			}
			w.update(func() {
				e.running, e.version, e.items = false, version, items
				w.insights.problemsGen++
				w.applyDiagnostics(path)
			})
		}()
	}
	for path := range w.insights.problems {
		if _, ok := w.docs[path]; !ok {
			delete(w.insights.problems, path)
			w.insights.problemsGen++
		}
	}
}

func (w *Workspace) applyDiagnostics(path string) {
	e, d := w.insights.problems[path], w.doc(path)
	if e == nil || d == nil || d.Buffer == nil {
		return
	}
	buf := d.Buffer
	ds := make([]editor.Diagnostic, 0, len(e.items))
	for _, p := range e.items {
		if p.Line >= buf.LineCount() {
			continue
		}
		start := buf.Offset(editor.Pos{Line: p.Line, Col: p.Col})
		end := start + 1
		if p.EndCol > p.Col {
			end = buf.Offset(editor.Pos{Line: p.Line, Col: p.EndCol})
		}
		ds = append(ds, editor.Diagnostic{Start: start, End: min(end, buf.Len()), Severity: severityOf(p.Severity), Message: p.Message, Source: p.Source})
	}
	for _, t := range w.tabs() {
		if t.Path == path && t.Editor != nil {
			t.Editor.SetDiagnostics(e.version, ds)
		}
	}
}

func severityOf(s problems.Severity) editor.Severity {
	switch s {
	case problems.Error:
		return editor.SeverityError
	case problems.Warning:
		return editor.SeverityWarning
	}
	return editor.SeverityInfo
}
