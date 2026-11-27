package gitui

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"strconv"

	"github.com/ZerGo0/editor/internal/design/filediff"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/pane"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const errNoCommit = "The commit was not found."

func (g *Git) openGroupAt(gr group, rel string, preview, focus bool) {
	id, title := "git-changes:"+groupIDs[gr], groupTitles[gr]
	g.openPatchTabWith(id, title, nil, patchOpen{reveal: rel, preview: preview, focus: focus, prepare: func(t *patchTab) {
		t.prepare = func() {
			rows := changeGroups(g.snap.status)[gr]
			t.quick = func() []patchItem { return rowItems(rows) }
			t.load = func(ctx context.Context, r repo) ([]patchItem, error) { return g.groupItems(ctx, r, gr, rows) }
		}
	}})
}

func (g *Git) groupItems(ctx context.Context, r repo, gr group, rows []changeRow) ([]patchItem, error) {
	counts := map[string]git.FileDiff{}
	if tracked := trackedPaths(rows); len(tracked) > 0 {
		var fds []git.FileDiff
		var err error
		if gr == groupStaged {
			fds, err = r.DiffStaged(ctx, tracked...)
		} else {
			fds, err = r.DiffWorkTree(ctx, tracked...)
		}
		if err != nil {
			return nil, err
		}
		for _, fd := range fds {
			counts[fd.Path()] = fd
		}
	}
	items := rowItems(rows)
	for i := range items {
		it := &items[i]
		row := rows[i]
		if fd, ok := counts[row.file.Path]; ok {
			it.adds, it.dels, it.counted, it.binary = diffCounts(fd), diffDels(fd), true, fd.Binary
			it.fingerprint = diffFingerprint(fd)
		} else if info, err := os.Stat(g.paths.abs(row.file.Path)); err == nil {
			it.size = info.Size()
			it.fingerprint = strconv.FormatInt(info.Size(), 36) + ":" + strconv.FormatInt(info.ModTime().UnixNano(), 36)
		}
	}
	return items, nil
}

func rowItems(rows []changeRow) []patchItem {
	items := make([]patchItem, len(rows))
	for i := range rows {
		row := rows[i]
		src := rowSource(row)
		items[i] = patchItem{title: row.file.Path, detail: row.file.OrigPath, code: row.code(), src: &src, row: &row}
	}
	return items
}

func trackedPaths(rows []changeRow) []string {
	var paths []string
	for _, r := range rows {
		if !r.untracked() {
			paths = append(paths, r.file.Path)
		}
	}
	return paths
}

func rowSource(r changeRow) diffSource {
	switch {
	case r.group == groupStaged:
		return diffSource{kind: diffStaged, path: r.file.Path, oldPath: r.file.OrigPath}
	case r.untracked():
		return diffSource{kind: diffUntracked, path: r.file.Path}
	}
	return diffSource{kind: diffWorkTree, path: r.file.Path}
}

func diffCounts(fd git.FileDiff) int { return countLines(fd, '+') }

func diffDels(fd git.FileDiff) int { return countLines(fd, '-') }

func countLines(fd git.FileDiff, kind byte) int {
	n := 0
	for _, h := range fd.Hunks {
		for _, l := range h.Lines {
			if l.Kind == kind {
				n++
			}
		}
	}
	return n
}

func diffFingerprint(fd git.FileDiff) string {
	h := fnv.New64a()
	for _, hunk := range fd.Hunks {
		fmt.Fprintf(h, "%d,%d,%d,%d;", hunk.OldStart, hunk.OldLines, hunk.NewStart, hunk.NewLines)
		for _, l := range hunk.Lines {
			_, _ = h.Write([]byte{l.Kind})
			_, _ = io.WriteString(h, l.Text)
		}
	}
	return strconv.FormatUint(h.Sum64(), 36)
}

func (g *Git) openCommitTab(cm git.Commit) {
	g.openCommitTabAt(cm, "", false, true)
}

func (g *Git) openCommitTabAt(cm git.Commit, rel string, preview, focus bool) {
	title := "Commit " + shortHash(cm.OID)
	if cm.Subject != "" {
		title = shortHash(cm.OID) + " " + cm.Subject
	}
	oid := cm.OID
	g.openPatchTabWith("git-commit:"+oid, title, func(ctx context.Context, r repo) ([]patchItem, error) {
		commits, err := r.Log(ctx, git.LogOptions{Revs: []string{oid}, Max: 1})
		if err != nil {
			return nil, err
		}
		if len(commits) == 0 {
			return nil, errors.New(errNoCommit)
		}
		full := commits[0]
		files, err := r.CommitFiles(ctx, oid)
		if err != nil {
			return nil, err
		}
		counts := map[string]git.FileDiff{}
		if len(full.Parents) > 0 {
			if fds, err := r.DiffCommits(ctx, full.Parents[0], oid); err == nil {
				for _, fd := range fds {
					counts[fd.Path()] = fd
				}
			}
		}
		items := []patchItem{{title: full.Subject, commit: &full, full: true}}
		for _, f := range files {
			src := commitSource(full, f)
			it := patchItem{title: f.Path, detail: f.OrigPath, code: f.Status, src: &src}
			if fd, ok := counts[f.Path]; ok {
				it.adds, it.dels, it.counted, it.binary = diffCounts(fd), diffDels(fd), true, fd.Binary
			}
			items = append(items, it)
		}
		return items, nil
	}, patchOpen{reveal: rel, preview: preview, focus: focus})
}

func (g *Git) commitSection(cm git.Commit, full bool) func(c *ui.Context) {
	if !full {
		return func(c *ui.Context) {
			list.Row(c, list.State{}, list.Item{Label: cm.Subject, Icon: icons.Commit, Description: shortHash(cm.OID)})
		}
	}
	return func(c *ui.Context) {
		ui.Column(c).FillWidth().Padding(0, 0, filediff.CardGap, 0).Children(func() {
			g.commitInfo(c, cm, nil, nil)
			ui.Row(c).Padding(0, token.PadView.Px(), token.PadView.Px(), token.PadView.Px()).Children(func() {
				if button(c, "Show in Log", false) {
					g.showCommit(cm.OID)
				}
			})
			pane.Separator(c, false)
		})
	}
}
