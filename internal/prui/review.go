package prui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/detail"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/ghshared"
	"github.com/ZerGo0/editor/internal/git/diffmode"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/multidiff"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	reviewFilesTitle = "Files Changed"
	reviewSinceTitle = "Changes Since Review"
)

type diffTab struct {
	data     *prData
	file     github.PRFile
	spec     diffSpec
	review   *reviewTab
	state    *editor.DiffState
	engine   *diffmode.Engine
	expanded map[string]bool
	oldSHA   string
}

type reviewTab struct {
	data         *prData
	spec         diffSpec
	id           string
	only         *github.PRFile
	view         *multidiff.View
	files        map[string]*diffTab
	hideComments bool
	mu           sync.Mutex
	base         string
}

func reviewID(repo github.Remote, number int, spec diffSpec, only string) string {
	switch {
	case spec.mode == modeLocal:
		return fmt.Sprintf("%s/%s/%s", detailID(repo, number), spec.mode, only)
	case spec.mode == modeSince:
		return fmt.Sprintf("%s/%s", detailID(repo, number), spec.mode)
	}
	return fmt.Sprintf("%s/%s", detailID(repo, number), diffIDSegment)
}

func reviewTitle(number int, spec diffSpec, only string) string {
	switch spec.mode {
	case modeSince:
		return fmt.Sprintf("#%d %s", number, reviewSinceTitle)
	case modeLocal:
		return fmt.Sprintf("%s (#%d, local)", filepath.Base(only), number)
	}
	return fmt.Sprintf("#%d %s", number, reviewFilesTitle)
}

func (p *PRs) openDiff(d *prData, f github.PRFile, spec diffSpec) {
	if spec.mode == "" {
		spec.mode = modePR
	}
	id := reviewID(p.repo, d.number, spec, f.Path)
	rt := p.diffs[id]
	if rt == nil {
		rt = &reviewTab{data: d, spec: spec, id: id, view: multidiff.New(), files: map[string]*diffTab{}}
		p.diffs[id] = rt
	}
	rt.only = nil
	if spec.mode == modeLocal {
		rt.only = &f
	}
	rt.base = d.baseSHA
	rt.view.SetEntries(p.reviewEntries(rt))
	rt.view.RevealPath(f.Path)
	p.Host.OpenView(id, reviewTitle(d.number, spec, f.Path), func(c *ui.Context) { p.reviewView(c, rt) })
}

func (p *PRs) reviewFiles(rt *reviewTab) []github.PRFile {
	switch {
	case rt.only != nil:
		return []github.PRFile{*rt.only}
	case rt.spec.mode == modeSince:
		return rt.data.since
	}
	return rt.data.files
}

func (p *PRs) closeReviewTabs() {
	for id, rt := range p.diffs {
		p.Host.CloseView(id)
		rt.view.Close()
	}
	for id := range p.logs {
		p.Host.CloseView(id)
	}
	clear(p.diffs)
	clear(p.logs)
}

func (p *PRs) reviewEntries(rt *reviewTab) []multidiff.Entry {
	files := p.reviewFiles(rt)
	entries := make([]multidiff.Entry, 0, len(files))
	for _, f := range files {
		t := rt.files[f.Path]
		if t == nil {
			t = &diffTab{data: rt.data, spec: rt.spec, review: rt, expanded: map[string]bool{}}
			rt.files[f.Path] = t
		}
		t.file = f
		entries = append(entries, p.reviewEntry(rt, t))
	}
	return entries
}

func (p *PRs) reviewEntry(rt *reviewTab, t *diffTab) multidiff.Entry {
	f, d := t.file, rt.data
	return multidiff.Entry{
		Key: f.Path, Path: f.Path, OldPath: f.PrevPath, Status: fileGitStatus(f.Status), Adds: f.Additions, Dels: f.Deletions,
		Fingerprint: d.pr.HeadSHA,
		Load:        func(ctx context.Context, _ bool) (string, string, error) { return p.loadReviewFile(ctx, rt, t) },
		Setup: func(st *editor.DiffState, eng *diffmode.Engine) {
			t.state, t.engine = st, eng
			st.LineSelection = true
			st.OnGutterButton = func(r editor.DiffLineRange) { p.commentAt(t, r) }
			eng.Install(func(c *ui.Context, a editor.DiffAnnotation) { p.threadView(c, t, a) })
		},
		Annotations: func(st *editor.DiffState) []editor.DiffAnnotation {
			if rt.spec.mode != modePR || rt.hideComments {
				return nil
			}
			return buildAnnotations(d.threads, f.Path, t.expanded, st.Old.LineCount(), st.New.LineCount())
		},
		Actions: func(c *ui.Context) { p.reviewActions(c, rt, t) },
	}
}

func (p *PRs) loadReviewFile(ctx context.Context, rt *reviewTab, t *diffTab) (string, string, error) {
	src := fileSource{local: p.local, be: p.be, repo: p.repo, root: p.Host.RootDir()}
	d, pr, file, spec := rt.data, rt.data.pr, t.file, rt.spec
	rt.mu.Lock()
	base := rt.base
	if base != "" {
		rt.mu.Unlock()
	}
	v, err := loadVersions(ctx, src, pr, file, spec, base)
	if base == "" {
		if err == nil && spec.mode == modePR {
			rt.base = v.baseSHA
		}
		rt.mu.Unlock()
	}
	p.Post(func() {
		if err != nil {
			p.AuthFailure(err)
			return
		}
		if spec.mode == modePR {
			d.baseSHA = v.baseSHA
		}
		t.oldSHA = v.baseSHA
	})
	if errors.Is(err, ErrBinaryFile) || errors.Is(err, ErrFileTooLarge) {
		return "", "", multidiff.Note(err.Error())
	}
	return v.old, v.new, err
}

func (p *PRs) reviewActions(c *ui.Context, rt *reviewTab, t *diffTab) {
	d := rt.data
	if rt.spec.mode != modePR {
		return
	}
	action.IconButton(c, icons.Comment, "Comment on File", action.IconOptions{Disabled: !d.isOpen(), ID: "comment-file"}).OnClick(func() { p.commentOnFile(t) })
	action.IconButton(c, icons.Split, "Compare with Local", action.IconOptions{ID: "compare-local"}).OnClick(func() { p.openDiff(d, t.file, diffSpec{mode: modeLocal}) })
	action.MenuIconButton(c, icons.More, "Copy or open a link", action.IconOptions{ID: "link"}, func(m *ui.Menu) { p.linkMenu(c, m, t) })
}

type serverViewed struct {
	p *PRs
	d *prData
}

func (s serverViewed) IsViewed(_ string, e *multidiff.Entry) bool { return s.d.viewed[e.Path] }

func (s serverViewed) SetViewed(_ string, e *multidiff.Entry, on bool) {
	s.p.setViewed(s.d, e.Path, on)
}

func (p *PRs) diffOptions(label, set string) multidiff.Options {
	return multidiff.Options{
		Label: label,
		Set:   set,
		Ctx:   p.Ctx,
		Post:  p.Post,
		Configure: func(st *editor.DiffState, rel string) {
			p.Host.ConfigureDiff(st, filepath.Join(p.Host.RootDir(), rel))
		},
		Language: func(rel, text string) *editor.Language { return p.Host.Language(rel, text) },
	}
}

func (p *PRs) reviewOptions(rt *reviewTab) multidiff.Options {
	d := rt.data
	o := p.diffOptions(reviewTitle(d.number, rt.spec, ""), rt.id)
	o.OpenFile = func(rel string) { p.Host.OpenAt(filepath.Join(p.Host.RootDir(), rel), -1, -1) }
	o.Busy = d.loading
	o.Toolbar = func(c *ui.Context) {
		if rt.spec.mode == modePR {
			action.Checkbox(c, "Hide Comments", &rt.hideComments, action.ChoiceOptions{ID: "hide-comments"})
		}
		ghshared.SmallButton(c, "Open Pull Request", false).OnClick(func() { p.openPR(d.number, nil) })
	}
	if rt.spec.mode == modePR && d.pr.NodeID != "" {
		o.Viewed = serverViewed{p: p, d: d}
	}
	return o
}

func (p *PRs) reviewView(c *ui.Context, rt *reviewTab) {
	if p.Closed {
		return
	}
	detail.Page(c, func() {
		if len(p.reviewFiles(rt)) == 0 {
			detail.Centered(c, func() { feedback.Empty(c, feedback.EmptyOptions{Icon: icons.File, Title: "No changes"}) })
			return
		}
		rt.view.Render(c, p.reviewOptions(rt))
	})
}
