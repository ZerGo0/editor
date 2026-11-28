package reviewui

import (
	"context"
	"path/filepath"

	"github.com/ZerGo0/editor/internal/multidiff"
	"github.com/ZerGo0/editor/internal/review"
	"github.com/egoist/mygo/ui/editor"
)

const planFileName = "plan.md"

type document struct {
	view *multidiff.View
	opts multidiff.Options
}

func newDocument(req review.Request, deps Deps, decorate Decorate) *document {
	name, abs := documentName(req), req.File
	read := func() (string, error) {
		if req.Kind == review.KindPlan {
			return req.Text, nil
		}
		return deps.ReadFile(req.File)
	}
	e := multidiff.Entry{Key: name, Title: name, Path: name, Whole: true,
		Load: func(context.Context, bool) (string, string, error) {
			text, err := read()
			return text, text, err
		}}
	d := &document{view: multidiff.New()}
	d.view.SetEntries(decorate([]multidiff.Entry{e}))
	d.opts = multidiff.Options{
		Label: name, Set: "review-document:" + name, Ctx: deps.Ctx, Post: deps.Post,
		Configure: func(st *editor.DiffState, _ string) { deps.Configure(st, abs) },
		Language:  func(_, text string) *editor.Language { return deps.Language(name, text) },
	}
	return d
}

func documentName(req review.Request) string {
	switch {
	case req.Kind == review.KindPlan && req.File == "":
		return planFileName
	case req.Kind == review.KindPlan:
		return filepath.Base(req.File)
	}
	if rel, err := filepath.Rel(req.Folder, req.File); err == nil && filepath.IsLocal(rel) {
		return filepath.ToSlash(rel)
	}
	return filepath.Base(req.File)
}

func (d *document) View() *multidiff.View { return d.view }

func (d *document) Options() multidiff.Options { return d.opts }

func (d *document) Status() (bool, bool, error) { return true, false, nil }

func (d *document) Reload() {}

func (d *document) Close() { d.view.Close() }
