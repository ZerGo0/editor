package gitui

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
)

const (
	detailsMaxHeight = 220
	containsShown    = 6
	filesSplit       = 55
)

type commitDetails struct {
	oid      string
	files    []git.ChangedFile
	contains []string
	loading  bool
	err      error
	list     ui.ListState
	selected int
	gen      int
	split    float32
}

func (s *logState) loadDetails(cm git.Commit) {
	g := s.g
	d := &s.details
	d.gen++
	gen := d.gen
	split := d.split
	if split == 0 {
		split = filesSplit
	}
	*d = commitDetails{oid: cm.OID, loading: true, selected: -1, gen: gen, split: split}
	r := g.repo
	go func() {
		files, err := r.CommitFiles(g.ctx, cm.OID)
		contains, _ := r.BranchesContaining(g.ctx, cm.OID)
		g.post(func() {
			if gen != d.gen {
				return
			}
			d.loading, d.err, d.files, d.contains = false, err, files, contains
		})
	}()
}

func commitSource(cm git.Commit, f git.ChangedFile) diffSource {
	src := diffSource{kind: diffCommit, path: f.Path, oldPath: f.OrigPath, rev: cm.OID}
	if len(cm.Parents) > 0 {
		src.parent = cm.Parents[0]
	}
	return src
}

func (s *logState) detailsView(c *ui.Context) {
	cm, ok := s.selectedCommit()
	if !ok {
		emptyState(c, "Select a commit", "Its changed files and details show here.")
		return
	}
	d := &s.details
	ui.SplitVerticalPercent(c, &d.split, func() { s.filesView(c, cm) }, func() {
		ui.Scroll(c).Fill().Children(func() { s.g.commitInfo(c, cm, d.contains, s.focusCommit) })
	}).Fill()
}

func (s *logState) filesView(c *ui.Context, cm git.Commit) {
	d := &s.details
	switch {
	case d.err != nil:
		errorState(c, "Cannot load the changed files", d.err, func() { s.loadDetails(cm) })
		return
	case d.loading:
		loadingState(c, "Loading files")
		return
	case len(d.files) == 0:
		emptyState(c, "No changed files", "")
		return
	}
	files := d.files
	d.list.Key = func(i int) any { return files[i].Path }
	d.list.Label = func(i int) string { return files[i].Path }
	d.list.Selected = &d.selected
	list := ui.List(c.Key("files"), &d.list, len(files)).Fill().Label("Changed Files")
	list.Rows(func(row ui.ListRow) {
		f := files[row.Index]
		el := s.g.fileRow(row.Context, f.Path, f.Status)
		if el.RowDoubleClicked() {
			s.g.openDiffSource(commitSource(cm, f))
		}
		el.ContextMenu(func(m *ui.Menu) { s.g.commitFileMenu(row.Context, m, cm, f) })
	})
	if list.Submitted() && d.selected >= 0 && d.selected < len(files) {
		s.g.openDiffSource(commitSource(cm, files[d.selected]))
	}
}

func (g *Git) fileRow(c *ui.Context, rel string, code byte) ui.Element {
	t := c.Theme()
	name, dir := splitPath(rel)
	el := ui.Row(c).Height(rowHeight).FillWidth().Padding(0, rowPadding).Gap(6).AlignItems(ui.Center).
		Tooltip(statusName(code) + ": " + rel)
	el.Children(func() {
		ui.Text(c, statusLetter(code)).FontSize(smallFont).Bold().TextColor(statusColor(t, code)).Width(statusWidth).Shrink(0)
		text := ui.Text(c, name).FontSize(bodyFont).SingleLine().Shrink(0)
		if code == 'D' {
			text.Strikethrough()
		}
		if dir != "" {
			muted(c, dir).SingleLine().Shrink(1).MinWidth(0).TruncateStart()
		}
	})
	return el
}

func (g *Git) commitFileMenu(c *ui.Context, m *ui.Menu, cm git.Commit, f git.ChangedFile) {
	src := commitSource(cm, f)
	abs := g.paths.abs(f.Path)
	if m.Item("Show Diff").Chosen() {
		g.openDiffSource(src)
	}
	if m.Item("Compare with Local").Chosen() {
		g.openDiffSource(diffSource{kind: diffCompare, path: f.Path, rev: cm.OID})
	}
	if m.Item("Compare Before with Local").Disabled(src.parent == "").Chosen() {
		g.openDiffSource(diffSource{kind: diffCompare, path: beforePath(f), rev: src.parent})
	}
	if m.Item("Open File").Chosen() {
		g.h.OpenAt(abs, -1, -1)
	}
	if m.Item("Revert Selected Changes").Disabled(src.parent == "").Chosen() {
		g.revertCommitFile(cm, f)
	}
	m.Separator()
	if m.Item("Show History").Chosen() {
		g.fileHistory(f.Path)
	}
	if m.Item("Copy Path").Chosen() {
		c.WriteClipboard(f.Path)
	}
}

func beforePath(f git.ChangedFile) string {
	if f.OrigPath != "" {
		return f.OrigPath
	}
	return f.Path
}

func (g *Git) revertCommitFile(cm git.Commit, f git.ChangedFile) {
	if len(cm.Parents) == 0 {
		g.h.Notify("The first commit has no parent to revert to.")
		return
	}
	parent, paths := cm.Parents[0], []string{f.Path}
	if f.OrigPath != "" {
		paths = append(paths, f.OrigPath)
	}
	g.doSimple("Revert Changes", func(ctx context.Context, r repo) error {
		fds, err := r.DiffCommits(ctx, parent, cm.OID, paths...)
		if err != nil {
			return err
		}
		for _, fd := range fds {
			patch := git.BuildPatch(fd, func(int, int) bool { return true }, false)
			if err := r.Apply(ctx, patch, false, true); err != nil {
				return fmt.Errorf("revert %s: %w", fd.Path(), err)
			}
		}
		return nil
	}, func() { g.reloadBuffers(paths) })
}

func (g *Git) commitInfo(c *ui.Context, cm git.Commit, contains []string, focus func(string)) {
	t := c.Theme()
	now := c.Now()
	ui.Column(c).FillWidth().Padding(rowPadding).Gap(6).Children(func() {
		ui.Text(c, cm.Subject).FontSize(bodyFont).Bold().Selectable()
		if body := strings.TrimSpace(cm.Body); body != "" {
			ui.Text(c, body).FontSize(smallFont).Selectable()
		}
		ui.Row(c).Gap(6).AlignItems(ui.Center).Wrap().Children(func() {
			ui.Text(c, cm.OID).FontSize(smallFont).TextColor(t.TextMuted).Selectable()
			if textButton(c, "Copy Hash", false).Clicked() {
				c.WriteClipboard(cm.OID)
			}
		})
		muted(c, fmt.Sprintf("%s <%s>, %s", cm.Author, cm.AuthorEmail, formatTime(cm.AuthorTime, now))).Selectable()
		if cm.Committer != "" && cm.Committer != cm.Author {
			muted(c, fmt.Sprintf("Committed by %s, %s", cm.Committer, formatTime(cm.CommitTime, now)))
		}
		if len(cm.Parents) > 0 {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Wrap().Children(func() {
				muted(c, "Parents:")
				for _, p := range cm.Parents {
					if textButton(c, shortHash(p), focus == nil).Clicked() && focus != nil {
						focus(p)
					}
				}
			})
		}
		if len(contains) > 0 {
			shown := contains[:min(len(contains), containsShown)]
			label := "In " + strings.Join(shown, ", ")
			if more := len(contains) - len(shown); more > 0 {
				label += fmt.Sprintf(" and %d more", more)
			}
			muted(c, label)
		}
	})
}
