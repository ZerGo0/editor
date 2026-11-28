package gitui

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ZerGo0/editor/internal/plural"
	"github.com/egoist/mygo/ui"
)

const (
	ignoreFileMode = 0o644
	excludeDirMode = 0o755
	excludeDir     = "info"
	excludeFile    = "exclude"
)

func (g *Git) changeMenu(c *ui.Context, m *ui.Menu, e changeEntry) {
	rows := g.targets(e)
	busy := g.busy != ""
	merge, staged, changes := rowsIn(rows, groupMerge), rowsIn(rows, groupStaged), rowsIn(rows, groupChanges)
	single := e.kind == entryFile && len(rows) == 1
	if single {
		r := rows[0]
		if r.group == groupMerge {
			if m.Item("Open Merge Editor").Chosen() {
				g.openMergeTool(r.file.Path)
			}
		} else if m.Item("Open Changes").Shortcut(showDiffMods, showDiffKey).Chosen() {
			g.openRowDiff(r)
		}
		if m.Item("Open File").Shortcut(0, jumpToSourceKey).Disabled(r.code() == codeDeleted).Chosen() {
			g.openFile(r)
		}
	} else if e.kind == entryGroup && e.group != groupMerge {
		if m.Item("Open Changes").Disabled(len(rows) == 0).Chosen() {
			g.openGroupChanges(e.group)
		}
	}
	m.Separator()
	if single && e.group == groupMerge {
		path := rows[0].file.Path
		if m.Item("Accept Yours").Disabled(busy).Chosen() {
			g.acceptSide(path, true, false)
		}
		if m.Item("Accept Theirs").Disabled(busy).Chosen() {
			g.acceptSide(path, false, false)
		}
	}
	if len(merge)+len(changes) > 0 && m.Item("Stage Changes").Disabled(busy).Chosen() {
		g.stage(rowPaths(append(merge, changes...))...)
	}
	if len(staged) > 0 && m.Item("Unstage Changes").Disabled(busy).Chosen() {
		g.unstage(rowPaths(staged)...)
	}
	if len(changes) > 0 && m.Item(discardLabel(changes)).Danger().Disabled(busy).Chosen() {
		g.rollback(changes)
	}
	if paths := shelvable(append(staged, changes...)); len(paths) > 0 && m.Item("Shelve Changes...").Disabled(busy).Chosen() {
		g.askShelve(paths)
	}
	if !single {
		return
	}
	r := rows[0]
	if r.untracked() {
		m.Separator()
		if m.Item("Add to .gitignore").Chosen() {
			g.addToIgnore(r.file.Path)
		}
		if m.Item("Add to .git/info/exclude").Chosen() {
			g.addToExclude(r.file.Path)
		}
	}
	m.Separator()
	if m.Item("Show History").Disabled(r.untracked()).Chosen() {
		g.fileHistory(r.file.Path)
	}
	if m.Item("Copy Path").Chosen() {
		c.WriteClipboard(g.paths.abs(r.file.Path))
	}
	if m.Item("Copy Relative Path").Chosen() {
		c.WriteClipboard(r.file.Path)
	}
}

func discardLabel(rows []changeRow) string {
	for _, r := range rows {
		if !r.untracked() {
			return "Discard Changes"
		}
	}
	return "Delete Files"
}

func shelvable(rows []changeRow) []string {
	seen := map[string]bool{}
	var paths []string
	for _, r := range rows {
		if !r.untracked() && !seen[r.file.Path] {
			seen[r.file.Path] = true
			paths = append(paths, r.file.Path)
		}
	}
	return paths
}

func (g *Git) rollback(rows []changeRow) {
	if len(rows) == 0 {
		return
	}
	p := planRollback(rows)
	q := question{
		message:     "Discard the changes in " + plural.Count(len(rows), "file", "files") + "?",
		detail:      "Your local changes are lost.",
		action:      "Discard",
		destructive: true,
	}
	if len(rows) == 1 {
		q.message = fmt.Sprintf("Discard the changes in %s?", rows[0].file.Path)
	}
	if len(p.untracked) == len(rows) {
		q.message, q.detail, q.action = "Delete "+plural.Count(len(rows), "untracked file", "untracked files")+"?", "The files are deleted from the disk.", "Delete"
	}
	g.confirm(q, func() {
		g.doSimple("Discard", func(ctx context.Context, r repo) error {
			if err := r.UnstageFiles(ctx, p.unstage...); err != nil {
				return err
			}
			if err := r.Discard(ctx, p.discard...); err != nil {
				return err
			}
			return r.DiscardUntracked(ctx, p.untracked...)
		}, func() { g.reloadBuffers(append(p.discard, p.untracked...)) })
	})
}

func (g *Git) reloadBuffers(paths []string) {
	for _, rel := range paths {
		abs := g.paths.abs(rel)
		buf := g.h.Buffer(abs)
		if buf == nil {
			continue
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		buf.Reset(string(data))
	}
}

func (g *Git) addToIgnore(rel string) {
	g.doSimple("Add to .gitignore", func(_ context.Context, r repo) error {
		return appendIgnoreFile(filepath.Join(r.WorkDir(), ignoreFileName), rel)
	}, nil)
}

func (g *Git) addToExclude(rel string) {
	g.doSimple("Add to .git/info/exclude", func(_ context.Context, r repo) error {
		dir := filepath.Join(r.MetaDir(), excludeDir)
		if err := os.MkdirAll(dir, excludeDirMode); err != nil {
			return err
		}
		return appendIgnoreFile(filepath.Join(dir, excludeFile), rel)
	}, nil)
}

func appendIgnoreFile(file, rel string) error {
	data, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read %s: %w", file, err)
	}
	if err := os.WriteFile(file, []byte(appendIgnore(string(data), rel)), ignoreFileMode); err != nil {
		return fmt.Errorf("write %s: %w", file, err)
	}
	return nil
}

func (g *Git) acceptSide(rel string, ours, reloadConflicts bool) {
	g.doSimple("Resolve", func(ctx context.Context, r repo) error { return r.AcceptSide(ctx, rel, ours) }, func() {
		g.reloadBuffers([]string{rel})
		if reloadConflicts {
			g.loadConflicts()
		}
	})
}
