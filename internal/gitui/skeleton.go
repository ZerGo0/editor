package gitui

import (
	"fmt"
	"time"

	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
)

const (
	skeletonCommits  = 10
	subjectWordsMin  = 3
	subjectWordsSpan = 4
	pathSeparator    = "/"
)

var skeletonCodes = []byte{codeModified, codeModified, codeAdded, codeModified, codeDeleted, codeUntracked}

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

func changesSkeleton(c *ui.Context, label string) {
	list.Skeleton(c, label, list.SkeletonRows+1, func(i int) list.Item {
		if i == 0 {
			return list.Item{Label: placeholderWord(i), Branch: true, Open: true, Strong: true, Count: list.SkeletonRows}
		}
		it := changedFileItem(placeholderPath(i), skeletonCodes[i%len(skeletonCodes)], nil)
		it.Depth = 1
		return it
	})
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
