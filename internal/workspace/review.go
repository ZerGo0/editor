package workspace

import (
	"os"
	"path/filepath"

	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/review"
	"github.com/ZerGo0/editor/internal/reviewui"
)

const reviewIDPrefix = "review:"

func (w *Workspace) OpenReview(s review.Session, req review.Request) {
	if w.reviews == nil {
		w.reviews = map[string]*reviewui.Tab{}
	}
	p := virtualPrefix + reviewIDPrefix + filepath.Base(s.Dir)
	if _, ok := w.reviews[p]; !ok {
		t := reviewui.New(req, w.reviewDeps(s, req))
		w.reviews[p] = t
		w.virtual[p] = &virtualTab{title: t.Title(), view: t.View, icon: icons.Comment, close: func() {
			t.Abandon()
			t.Close()
			delete(w.reviews, p)
		}}
	}
	w.OpenFile(p, false, true)
}

func (w *Workspace) reviewDeps(s review.Session, req review.Request) reviewui.Deps {
	d := reviewui.Deps{
		Ctx:       w.ctx,
		Post:      w.Update,
		Decide:    func(res review.Result) error { return s.Decide(req, res) },
		Configure: w.ConfigureDiff,
		Language:  w.Language,
		ReadFile:  readText,
	}
	if w.git != nil {
		d.Changes = func(mode review.DiffMode, base string, decorate reviewui.Decorate) reviewui.Changes {
			return w.git.ReviewChanges(mode, base, decorate)
		}
	}
	if w.prs != nil {
		d.PR = func(ref review.PRRef, decorate reviewui.Decorate) reviewui.PullRequest {
			return w.prs.AgentReview(ref, decorate)
		}
	}
	return d
}

func readText(path string) (string, error) {
	data, err := os.ReadFile(path)
	return string(data), err
}

func (w *Workspace) activeReview() *reviewui.Tab {
	if t := w.active.ActiveTab(); t != nil {
		return w.reviews[t.Path]
	}
	return nil
}

func (w *Workspace) RunReview(id string) {
	t := w.activeReview()
	if t == nil {
		return
	}
	switch id {
	case reviewui.DecideID:
		if !t.Composing() {
			t.Decide()
		}
	case reviewui.ApproveWithNotesID:
		if !t.Composing() {
			t.ApproveWithNotes()
		}
	case reviewui.CloseID:
		t.Dismiss()
	case reviewui.AddGlobalCommentID:
		t.AddGlobalComment()
	}
}

func (w *Workspace) abandonReviews() {
	for _, t := range w.reviews {
		t.Abandon()
	}
}
