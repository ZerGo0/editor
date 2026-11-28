package gitui

import (
	"fmt"
	"time"

	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/multidiff"
	"github.com/egoist/mygo/ui"
)

const (
	skeletonCommits  = 10
	subjectWordsMin  = 3
	subjectWordsSpan = 4
	pathSeparator    = "/"
	statusUnchanged  = '.'

	mergeSkeletonLead     = 8
	mergeSkeletonConflict = 3
)

var (
	skeletonCodes  = []byte{codeModified, codeModified, codeAdded, codeModified, codeDeleted, codeUntracked}
	skeletonCounts = []int{3, 1, 5, 2, 4, 1}
)

func boxedSkeleton(c *ui.Context, label string, item func(i int) list.Item) {
	boxFrame(c, func() { list.Skeleton(c, label, list.SkeletonRows, item) })
}

func placeholderWord(i int) string { return feedback.Placeholder(i, 1) }

func placeholderPath(i int) string { return placeholderWord(i+1) + pathSeparator + placeholderWord(i) }

func placeholderSubject(i int) string {
	return feedback.Placeholder(i, subjectWordsMin+i%subjectWordsSpan)
}

func placeholderOID(i int) string { return fmt.Sprintf("%040x", i+1) }

func placeholderTime(now time.Time, i int) time.Time { return now.Add(-time.Duration(i+1) * time.Hour) }

func placeholderCommits(now time.Time) []git.Commit {
	commits := make([]git.Commit, skeletonCommits)
	for i := range commits {
		commits[i] = git.Commit{OID: placeholderOID(i), Subject: placeholderSubject(i), Author: placeholderWord(i + 2), AuthorTime: placeholderTime(now, i)}
		if i+1 < len(commits) {
			commits[i].Parents = []string{placeholderOID(i + 1)}
		}
	}
	return commits
}

func filesSkeleton(c *ui.Context, label string) ui.Element {
	return list.Skeleton(c, label, list.SkeletonRows, func(i int) list.Item {
		return changedFileItem(placeholderPath(i), skeletonCodes[i%len(skeletonCodes)], nil)
	})
}

func placeholderStatus() *git.Status {
	st := &git.Status{Files: make([]git.FileStatus, list.SkeletonRows)}
	for i := range st.Files {
		f := git.FileStatus{Path: placeholderPath(i), Index: statusUnchanged, WorkTree: skeletonCodes[i%len(skeletonCodes)]}
		if f.WorkTree == codeUntracked {
			f.Index = codeUntracked
		}
		st.Files[i] = f
	}
	return st
}

func (g *Git) changesSkeleton(c *ui.Context, label string) {
	entries := flattenChanges(changeGroups(placeholderStatus()), !g.changes.listMode, nil)
	feedback.Skeleton(c, label, func() {
		g.commitBox(c)
		list.Rows(c, len(entries), func(i int) list.Item { return g.changeItem(c, entries[i]) })
	})
}

func placeholderEntries() []multidiff.Entry {
	entries := make([]multidiff.Entry, list.SkeletonRows)
	for i := range entries {
		p := placeholderPath(i)
		entries[i] = multidiff.Entry{Key: p, Path: p, Status: gitStatus(skeletonCodes[i%len(skeletonCodes)]),
			Adds: skeletonCounts[i%len(skeletonCounts)], Dels: skeletonCounts[(i+1)%len(skeletonCounts)]}
	}
	return entries
}

func (t *patchTab) skeleton(c *ui.Context) {
	if t.placeholder == nil {
		t.placeholder = t.g.newDiffView()
		t.placeholder.SetEntries(placeholderEntries())
	}
	feedback.Skeleton(c, "Loading changes", func() { t.placeholder.Render(c, t.options()) }).Grow(1)
}

func placeholderMerge() (base, ours, theirs string) {
	lead := feedback.SkeletonCode(0, mergeSkeletonLead, feedback.SkeletonBase)
	tail := feedback.SkeletonCode(mergeSkeletonLead+mergeSkeletonConflict, feedback.SkeletonCodeLines-mergeSkeletonLead-mergeSkeletonConflict, feedback.SkeletonBase)
	side := func(version int) string {
		return lead + feedback.SkeletonCode(mergeSkeletonLead, mergeSkeletonConflict, version) + tail
	}
	return side(feedback.SkeletonBase), side(feedback.SkeletonOld), side(feedback.SkeletonNew)
}

func (t *mergeTool) skeleton(c *ui.Context) {
	if t.placeholder == nil {
		t.placeholder = &mergeTool{g: t.g, rel: t.rel, rebasing: t.rebasing}
		t.placeholder.setup(placeholderMerge())
	}
	feedback.Skeleton(c, "Loading conflict", func() {
		t.placeholder.toolbar(c)
		t.placeholder.panes(c)
	}).Grow(1)
}

func (g *Git) graphSkeleton(c *ui.Context) {
	commits := placeholderCommits(c.Now())
	rows := git.Lanes(commits)
	gw := graphWidth(rows)
	list.Skeleton(c, "Loading commits", len(commits), func(i int) list.Item { return g.graphCommitItem(c, commits, rows, i, gw, nil) })
}

func todoSkeleton(c *ui.Context, label string) {
	options := todoOptions()
	boxFrame(c, func() {
		feedback.Skeleton(c, label, func() {
			for i := range list.SkeletonRows {
				choice := string(git.ActionPick)
				rc := c.Key(i)
				todoRow(rc).Children(func() { todoRowContent(rc, placeholderOID(i), placeholderSubject(i), &choice, options) })
			}
		})
	})
}

func pushSkeleton(c *ui.Context, label string) {
	pushColumns(c, func() {
		pushColumn(c, func() {
			list.Skeleton(c, label, list.SkeletonRows, func(i int) list.Item {
				return pushCommitItem(c, git.Commit{OID: placeholderOID(i), Subject: placeholderSubject(i), Author: placeholderWord(i), AuthorTime: placeholderTime(c.Now(), i)})
			})
		})
		pushColumn(c, func() { filesSkeleton(c, label) })
	})
}
