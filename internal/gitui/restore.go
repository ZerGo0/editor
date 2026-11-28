package gitui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
)

const (
	diffViewPrefix    = "git-diff:"
	changesViewPrefix = "git-changes:"
	commitViewPrefix  = "git-commit:"
	historyViewPrefix = "git-history:"
	linesViewPrefix   = "git-lines:"
	diffIDParts       = 4
)

func (g *Git) registerRestorers() {
	g.h.RestoreViews(diffViewPrefix, g.restoreDiff)
	g.h.RestoreViews(changesViewPrefix, g.restoreGroup)
	g.h.RestoreViews(commitViewPrefix, g.restoreCommit)
	g.h.RestoreViews(historyViewPrefix, g.restoreHistory)
	g.h.RestoreViews(linesViewPrefix, g.restoreLines)
}

func parseDiffID(id string) (diffSource, bool) {
	rest, ok := strings.CutPrefix(id, diffViewPrefix)
	if !ok {
		return diffSource{}, false
	}
	parts := strings.SplitN(rest, ":", diffIDParts)
	if len(parts) != diffIDParts || parts[3] == "" {
		return diffSource{}, false
	}
	kind, err := strconv.Atoi(parts[0])
	if err != nil || kind < int(diffWorkTree) || kind > int(diffCompare) {
		return diffSource{}, false
	}
	return diffSource{kind: diffKind(kind), rev: parts[1], oldPath: parts[2], path: parts[3]}, true
}

func (g *Git) restoreDiff(id, _ string) (func(c *ui.Context), bool) {
	src, ok := parseDiffID(id)
	if !ok {
		return nil, false
	}
	return g.diffTabFor(src).view, true
}

func (g *Git) restoreGroup(id, _ string) (func(c *ui.Context), bool) {
	for gr, name := range groupIDs {
		if id == changesViewPrefix+name {
			t := g.patchTabFor(id, groupTitles[gr])
			g.prepareGroup(t, group(gr))
			return t.render, true
		}
	}
	return nil, false
}

func (g *Git) restoreCommit(id, title string) (func(c *ui.Context), bool) {
	oid, _ := strings.CutPrefix(id, commitViewPrefix)
	if oid == "" {
		return nil, false
	}
	t := g.patchTabFor(id, title)
	t.load = commitLoader(oid)
	return t.render, true
}

func (g *Git) restoreHistory(id, _ string) (func(c *ui.Context), bool) {
	rel, _ := strings.CutPrefix(id, historyViewPrefix)
	if rel == "" {
		return nil, false
	}
	return g.historyView(id, rel), true
}

func (g *Git) restoreLines(id, title string) (func(c *ui.Context), bool) {
	rest, _ := strings.CutPrefix(id, linesViewPrefix)
	i := strings.LastIndex(rest, ":")
	if i <= 0 {
		return nil, false
	}
	var start, last int
	if _, err := fmt.Sscanf(rest[i+1:], "%d-%d", &start, &last); err != nil || start <= 0 || last < start {
		return nil, false
	}
	t := g.patchTabFor(id, title)
	t.load = lineLoader(rest[:i], start, last)
	return t.render, true
}

func commitParent(ctx context.Context, r repo, rev string) string {
	commits, err := r.Log(ctx, git.LogOptions{Revs: []string{rev}, Max: 1})
	if err != nil || len(commits) == 0 || len(commits[0].Parents) == 0 {
		return ""
	}
	return commits[0].Parents[0]
}
