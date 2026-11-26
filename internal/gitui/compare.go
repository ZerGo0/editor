package gitui

import (
	"context"
	"fmt"

	"github.com/ZerGo0/editor/internal/git"
)

const compareCommitLimit = 200

func (g *Git) showCommit(oid string) {
	if !g.ready() {
		return
	}
	g.wantsPanel = true
	g.log.focusCommit(oid)
}

func (g *Git) showLog() {
	if !g.ready() {
		return
	}
	g.wantsPanel = true
}

func (g *Git) openCommitTab(cm git.Commit) {
	g.openPatchTab("git-commit:"+cm.OID, "Commit "+shortHash(cm.OID), func(ctx context.Context, r repo) ([]patchItem, error) {
		files, err := r.CommitFiles(ctx, cm.OID)
		if err != nil {
			return nil, err
		}
		items := []patchItem{{title: cm.Subject, commit: &cm}}
		for _, f := range files {
			src := commitSource(cm, f)
			items = append(items, patchItem{title: f.Path, detail: f.OrigPath, code: f.Status, src: &src})
		}
		return items, nil
	})
}

func (g *Git) compareWithLocal(rev string) {
	label := rev
	if isHashLike(rev) {
		label = shortHash(rev)
	}
	g.openPatchTab("git-compare-local:"+rev, label+" vs Local", func(ctx context.Context, r repo) ([]patchItem, error) {
		files, err := r.ChangedSince(ctx, rev)
		if err != nil {
			return nil, err
		}
		items := make([]patchItem, 0, len(files))
		for _, f := range files {
			src := diffSource{kind: diffCompare, path: f.Path, oldPath: f.OrigPath, rev: rev}
			items = append(items, patchItem{title: f.Path, detail: f.OrigPath, code: f.Status, src: &src})
		}
		return items, nil
	})
}

func (g *Git) compareBranches(base, other string) {
	title := other + " vs " + base
	g.openPatchTab("git-compare:"+base+"..."+other, title, func(ctx context.Context, r repo) ([]patchItem, error) {
		cmp, err := r.CompareBranches(ctx, base, other)
		if err != nil {
			return nil, err
		}
		fds, err := r.DiffCommits(ctx, base, other)
		if err != nil {
			return nil, err
		}
		var items []patchItem
		add := func(name string, commits []git.Commit) {
			items = append(items, patchItem{header: true, title: fmt.Sprintf("Commits only in %s", name), detail: fmt.Sprintf("%d commits", len(commits))})
			for i := range commits[:min(len(commits), compareCommitLimit)] {
				cm := commits[i]
				items = append(items, patchItem{title: cm.Subject, commit: &cm})
			}
		}
		add(other, cmp.BOnly)
		add(base, cmp.AOnly)
		items = append(items, patchItem{header: true, title: "Changed Files", detail: fmt.Sprintf("%d files", len(fds))})
		return append(items, fileDiffItems(fds)...), nil
	})
}

func (g *Git) compareFileWith(ref string) {
	rel, ok := g.activeRel()
	if !ok {
		return
	}
	g.openDiffSource(diffSource{kind: diffCompare, path: rel, rev: ref})
}

func (g *Git) showActiveDiff() {
	if rel, ok := g.activeRel(); ok {
		g.openDiff(diffHead, rel)
	}
}
