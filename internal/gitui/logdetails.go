package gitui

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	containsShown = 6
	filesSplit    = 55

	selectCommitHint = "Select a commit to see its files and details"
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
	applog.Go(func() {
		files, err := r.CommitFiles(g.ctx, cm.OID)
		contains, _ := r.BranchesContaining(g.ctx, cm.OID)
		g.post(func() {
			if gen != d.gen {
				return
			}
			d.loading, d.err, d.files, d.contains = false, err, files, contains
		})
	})
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
		ui.Column(c).Fill().Padding(token.PadPanel.Px()).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			secondary(c, selectCommitHint).SingleLine().MinWidth(0).Tooltip(selectCommitHint)
		})
		return
	}
	d := &s.details
	ui.SplitVerticalPercent(c, &d.split, func() { s.filesView(c, cm) }, func() {
		ui.Scroll(c).Fill().Children(func() { s.g.commitInfo(c, cm, d.contains, s.focusCommit) })
	}).SplitReset(filesSplit).Fill()
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
	l := list.View(c.Key("files"), &d.list, len(files), list.ViewOptions{Label: "Changed Files"}, func(i int) list.Item {
		f := files[i]
		it := changedFileItem(f.Path, f.Status, func(preview, focus bool) { s.g.openDiffWith(commitSource(cm, f), preview, focus) })
		it.Menu = func(m *ui.Menu) { s.g.commitFileMenu(c, m, cm, f) }
		return it
	})
	if l.Submitted() && d.selected >= 0 && d.selected < len(files) {
		s.g.openDiffWith(commitSource(cm, files[d.selected]), false, true)
	}
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
	p := token.Of(c)
	now := c.Now()
	ui.Column(c).FillWidth().Padding(token.PadView.Px()).Gap(token.GapStack.Px()).Children(func() {
		token.TextStrong.Apply(ui.Text(c, cm.Subject)).TextColor(p.Text.Primary).Wrap().Selectable()
		if body := strings.TrimSpace(cm.Body); body != "" {
			paragraph(c, body).Selectable()
		}
		ui.Row(c).Gap(token.GapInline.Px()).AlignItems(ui.Center).Wrap().Children(func() {
			token.TextMono.Apply(ui.Text(c, cm.OID)).TextColor(p.Text.Secondary).Selectable()
			if iconButton(c, "Copy Hash", icons.Copy, false) {
				c.WriteClipboard(cm.OID)
			}
		})
		secondary(c, fmt.Sprintf("%s <%s>, %s", cm.Author, cm.AuthorEmail, formatTime(cm.AuthorTime, now))).Wrap().Selectable()
		if cm.Committer != "" && cm.Committer != cm.Author {
			secondary(c, fmt.Sprintf("Committed by %s, %s", cm.Committer, formatTime(cm.CommitTime, now))).Wrap()
		}
		if len(cm.Parents) > 0 {
			ui.Row(c).Gap(token.GapInline.Px()).AlignItems(ui.Center).Wrap().Children(func() {
				secondary(c, "Parents:")
				for _, parent := range cm.Parents {
					if smallButton(c, shortHash(parent), focus == nil) && focus != nil {
						focus(parent)
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
			secondary(c, label).Wrap()
		}
	})
}
