package gitui

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/egoist/mygo/ui"
)

const (
	ignoreFileMode = 0o644
	excludeDirMode = 0o755
	excludeDir     = "info"
	excludeFile    = "exclude"
)

func (g *Git) fileMenu(c *ui.Context, m *ui.Menu, r changeRow) {
	abs := g.paths.abs(r.file.Path)
	if m.Item("Show Diff").Chosen() {
		g.openRowDiff(r)
	}
	if m.Item("Open File").Disabled(r.code() == 'D').Chosen() {
		g.h.OpenAt(abs, -1, -1)
	}
	m.Separator()
	switch r.group {
	case groupStaged:
		if m.Item("Unstage").Chosen() {
			g.unstage(r.file.Path)
		}
	case groupUnstaged, groupUntracked:
		if m.Item("Stage").Chosen() {
			g.stage(r.file.Path)
		}
	case groupConflicts:
		if m.Item("Merge...").Chosen() {
			g.openMergeTool(r.file.Path)
		}
		if m.Item("Accept Yours").Chosen() {
			g.acceptSide(r.file.Path, true)
		}
		if m.Item("Accept Theirs").Chosen() {
			g.acceptSide(r.file.Path, false)
		}
	}
	if r.group != groupConflicts {
		label := "Rollback..."
		if r.group == groupUntracked {
			label = "Delete..."
		}
		if m.Item(label).Danger().Chosen() {
			g.rollback([]changeRow{r})
		}
		if m.Item("Shelve Changes...").Disabled(r.group == groupUntracked).Chosen() {
			g.askShelve([]string{r.file.Path})
		}
	}
	if r.group == groupUntracked {
		if m.Item("Add to .gitignore").Chosen() {
			g.addToIgnore(r.file.Path)
		}
		if m.Item("Add to .git/info/exclude").Chosen() {
			g.addToExclude(r.file.Path)
		}
	}
	m.Separator()
	if m.Item("Show History").Disabled(r.group == groupUntracked).Chosen() {
		g.fileHistory(r.file.Path)
	}
	if m.Item("Copy Path").Chosen() {
		c.WriteClipboard(abs)
	}
	if m.Item("Copy Relative Path").Chosen() {
		c.WriteClipboard(r.file.Path)
	}
}

type rollbackPlan struct {
	unstage   []string
	discard   []string
	untracked []string
}

func planRollback(rows []changeRow) rollbackPlan {
	var p rollbackPlan
	for _, r := range rows {
		f := r.file
		switch {
		case r.group == groupUntracked:
			p.untracked = append(p.untracked, f.Path)
		case r.group == groupConflicts:
		case r.group == groupUnstaged:
			p.discard = append(p.discard, f.Path)
		case f.Index == 'A':
			p.unstage = append(p.unstage, f.Path)
		case f.OrigPath != "":
			p.unstage = append(p.unstage, f.Path, f.OrigPath)
			p.discard = append(p.discard, f.OrigPath)
		default:
			p.unstage = append(p.unstage, f.Path)
			p.discard = append(p.discard, f.Path)
		}
	}
	return p
}

func (g *Git) rollback(rows []changeRow) {
	if len(rows) == 0 {
		return
	}
	p := planRollback(rows)
	q := question{
		message:     fmt.Sprintf("Roll back the changes in %d files?", len(rows)),
		detail:      "Your local changes are lost.",
		action:      "Rollback",
		destructive: true,
	}
	if len(rows) == 1 {
		q.message = fmt.Sprintf("Roll back the changes in %s?", rows[0].file.Path)
	}
	if len(p.untracked) == len(rows) {
		q.message, q.detail, q.action = fmt.Sprintf("Delete %d untracked files?", len(rows)), "The files are deleted from the disk.", "Delete"
	}
	g.confirm(q, func() {
		g.doSimple("Rollback", func(ctx context.Context, r repo) error {
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

func (g *Git) acceptSide(rel string, ours bool) {
	g.doSimple("Resolve", func(ctx context.Context, r repo) error { return r.AcceptSide(ctx, rel, ours) },
		func() { g.reloadBuffers([]string{rel}) })
}
