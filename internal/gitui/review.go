package gitui

import (
	"context"
	"errors"

	"github.com/ZerGo0/editor/internal/multidiff"
	"github.com/ZerGo0/editor/internal/review"
)

var errNoRepository = errors.New("This folder is not a Git repository.")

type ReviewChanges struct {
	g    *Git
	t    *patchTab
	mode review.DiffMode
	base string
}

func (g *Git) ReviewChanges(mode review.DiffMode, base string, decorate func([]multidiff.Entry) []multidiff.Entry) *ReviewChanges {
	rc := &ReviewChanges{g: g, mode: mode, base: base}
	rc.t = &patchTab{g: g, id: "git-review:" + string(mode), title: review.DescribeDiff(mode, base), view: multidiff.New(), decorate: decorate}
	rc.t.prepare = rc.prepare
	g.reviews = append(g.reviews, rc.t)
	if g.snap.status != nil {
		rc.t.reload()
	}
	return rc
}

func (rc *ReviewChanges) prepare() {
	g, t := rc.g, rc.t
	rows := changeGroups(g.snap.status)
	untracked := untrackedRows(rows[groupChanges])
	switch rc.mode {
	case review.DiffStaged:
		t.load = func(ctx context.Context, r repo) ([]patchItem, error) {
			return g.groupItems(ctx, r, groupStaged, rows[groupStaged])
		}
	case review.DiffUnstaged:
		t.load = func(ctx context.Context, r repo) ([]patchItem, error) {
			return g.groupItems(ctx, r, groupChanges, rows[groupChanges])
		}
	case review.DiffLastCommit:
		t.load = func(ctx context.Context, r repo) ([]patchItem, error) { return commitItems(ctx, r, headRev) }
	case review.DiffBranch:
		base := rc.base
		t.load = func(ctx context.Context, r repo) ([]patchItem, error) {
			if base == "" {
				base = r.DefaultBranch(ctx)
				g.post(func() { rc.setBase(base) })
			}
			mergeBase, err := r.MergeBase(ctx, base, headRev)
			if err != nil {
				return nil, err
			}
			return g.sinceItems(ctx, r, diffSource{kind: diffCompare, rev: mergeBase}, untracked)
		}
	default:
		t.load = func(ctx context.Context, r repo) ([]patchItem, error) {
			return g.sinceItems(ctx, r, diffSource{kind: diffHead, rev: headRev}, untracked)
		}
	}
}

func (rc *ReviewChanges) setBase(base string) {
	rc.base = base
	rc.t.title = review.DescribeDiff(rc.mode, base)
}

func untrackedRows(rows []changeRow) []changeRow {
	var out []changeRow
	for _, r := range rows {
		if r.untracked() {
			out = append(out, r)
		}
	}
	return out
}

func (g *Git) sinceItems(ctx context.Context, r repo, src diffSource, untracked []changeRow) ([]patchItem, error) {
	files, err := r.ChangedSince(ctx, src.rev)
	if err != nil {
		return nil, err
	}
	items := make([]patchItem, 0, len(files)+len(untracked))
	for _, f := range files {
		s := src
		s.path, s.oldPath = f.Path, f.OrigPath
		it := patchItem{title: f.Path, detail: f.OrigPath, code: f.Status, src: &s}
		g.statItem(&it, f.Path)
		items = append(items, it)
	}
	extra, err := g.groupItems(ctx, r, groupChanges, untracked)
	return append(items, extra...), err
}

func (rc *ReviewChanges) View() *multidiff.View { return rc.t.view }

func (rc *ReviewChanges) Options() multidiff.Options { return rc.t.options() }

func (rc *ReviewChanges) Base() string { return rc.base }

func (rc *ReviewChanges) Title() string { return rc.t.title }

func (rc *ReviewChanges) Status() (loaded, loading bool, err error) {
	switch rc.g.phase {
	case phaseNoGit, phaseNoRepo:
		return true, false, errNoRepository
	case phaseError:
		return true, false, rc.g.openErr
	}
	return rc.t.loaded, rc.t.loading, rc.t.err
}

func (rc *ReviewChanges) Reload() { rc.t.reload() }

func (rc *ReviewChanges) Close() {
	g := rc.g
	for i, t := range g.reviews {
		if t == rc.t {
			g.reviews = append(g.reviews[:i], g.reviews[i+1:]...)
			break
		}
	}
	rc.t.gen++
	rc.t.view.Close()
}
