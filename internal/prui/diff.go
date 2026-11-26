package prui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/git/diffmode"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	modePR        = "pr"
	modeSince     = "since"
	modeLocal     = "local"
	layoutInline  = 1
	threadPadX    = 10
	threadPadY    = 6
	threadGap     = 4
	noPRInTabHint = "This pull request is not open, so comments cannot be added."
)

type diffSpec struct {
	mode   string
	oldRef string
}

type diffTab struct {
	data         *prData
	file         github.PRFile
	spec         diffSpec
	id           string
	state        *editor.DiffState
	engine       *diffmode.Engine
	loading      bool
	ready        bool
	err          error
	gen          int
	layout       int
	hideComments bool
	expanded     map[string]bool
	oldSHA       string
}

func diffTabID(repo github.Remote, number int, filePath string, spec diffSpec) string {
	if spec.mode == modePR || spec.mode == "" {
		return diffID(repo, number, filePath)
	}
	return fmt.Sprintf("%s/%s/%s", detailID(repo, number), spec.mode, filePath)
}

func diffTitle(number int, filePath string, spec diffSpec) string {
	switch spec.mode {
	case modeSince:
		return fmt.Sprintf("%s (#%d, since review)", path.Base(filePath), number)
	case modeLocal:
		return fmt.Sprintf("%s (#%d, local)", path.Base(filePath), number)
	}
	return fmt.Sprintf("%s (#%d)", path.Base(filePath), number)
}

func (p *PRs) openDiff(d *prData, f github.PRFile, spec diffSpec) {
	if spec.mode == "" {
		spec.mode = modePR
	}
	id := diffTabID(p.repo, d.number, f.Path, spec)
	t := p.diffs[id]
	if t == nil {
		t = &diffTab{data: d, file: f, spec: spec, id: id, expanded: map[string]bool{}}
		p.diffs[id] = t
	}
	t.file = f
	if !t.ready && !t.loading {
		p.loadDiff(t)
	}
	p.h.OpenView(id, diffTitle(d.number, f.Path, spec), func(c *ui.Context) { p.diffView(c, t) })
}

func (p *PRs) closeReviewTabs() {
	for id := range p.diffs {
		p.h.CloseView(id)
	}
	for id := range p.logs {
		p.h.CloseView(id)
	}
	clear(p.diffs)
	clear(p.logs)
}

func (p *PRs) loadDiff(t *diffTab) {
	t.gen++
	gen := t.gen
	t.loading, t.err = true, nil
	d := t.data
	src := fileSource{local: p.local, be: p.be, repo: p.repo, root: p.h.RootDir()}
	pr, file, spec, baseSHA := d.pr, t.file, t.spec, d.baseSHA
	go func() {
		v, err := loadVersions(p.ctx, src, pr, file, spec, baseSHA)
		p.post(func() {
			if gen == t.gen {
				p.applyDiff(t, v, err)
			}
		})
	}()
}

func (p *PRs) applyDiff(t *diffTab, v fileVersions, err error) {
	t.loading = false
	if err != nil {
		t.err = err
		p.authFailure(err)
		return
	}
	if t.spec.mode == modePR {
		t.data.baseSHA = v.baseSHA
	}
	t.oldSHA = v.baseSHA
	if t.engine != nil {
		t.engine.Close()
	}
	t.state = &editor.DiffState{
		LineSelection:  true,
		OnGutterButton: func(r editor.DiffLineRange) { p.commentAt(t, r) },
	}
	t.engine = diffmode.NewEngine(p.ctx, t.state, p.post)
	t.engine.Install(func(c *ui.Context, a editor.DiffAnnotation) { p.threadView(c, t, a) })
	t.engine.SetSource(diffmode.Source{Old: v.old, New: v.new, Language: p.h.Language(t.file.Path, v.new)})
	t.ready = true
}

func (t *diffTab) selectedCode(r editor.DiffLineRange) string {
	if r.Side != editor.DiffNew || t.state == nil {
		return ""
	}
	lines := make([]string, 0, r.End-r.Start+1)
	for i := r.Start; i <= r.End && i < t.state.New.LineCount(); i++ {
		lines = append(lines, t.state.New.Line(i))
	}
	return strings.Join(lines, "\n")
}

func (p *PRs) ensureReview(ctx context.Context, d *prData, pending string) (string, error) {
	if pending != "" {
		return pending, nil
	}
	return p.be.StartReview(ctx, p.repo, d.number)
}

func (p *PRs) commentAt(t *diffTab, r editor.DiffLineRange) {
	d := t.data
	if !d.isOpen() {
		p.h.Notify(noPRInTabHint)
		return
	}
	if t.spec.mode != modePR {
		p.h.Notify("Open the file from the changed files of the pull request to comment on it.")
		return
	}
	pending := d.pendingReview
	opts := lineComment(r, t.file.Path, "")
	where := fmt.Sprintf("%s:%d", t.file.Path, opts.Line)
	p.openDialog(dialog{
		title:       "Comment on " + where,
		hint:        "The comment is added to your pending review. Submit the review from the pull request tab.",
		placeholder: "Leave a comment",
		action:      "Add Review Comment",
		suggest:     t.selectedCode(r),
		validate:    requireBody("Write a comment first."),
		submit: func(ctx context.Context, in dialogInput) (func(), error) {
			id, err := p.ensureReview(ctx, d, pending)
			if err != nil {
				return nil, err
			}
			o := opts
			o.Body = in.body
			thread, err := p.be.AddReviewComment(ctx, id, o)
			if err != nil {
				return nil, err
			}
			return func() {
				d.pendingReview = id
				d.threads = append(d.threads, thread)
			}, nil
		},
	})
}

func (p *PRs) commentOnFile(t *diffTab) {
	d := t.data
	if !d.isOpen() {
		p.h.Notify(noPRInTabHint)
		return
	}
	pending, filePath := d.pendingReview, t.file.Path
	p.openDialog(dialog{
		title:       "Comment on " + filePath,
		hint:        "The comment is added to your pending review and applies to the whole file.",
		placeholder: "Leave a comment",
		action:      "Add File Comment",
		validate:    requireBody("Write a comment first."),
		submit: func(ctx context.Context, in dialogInput) (func(), error) {
			id, err := p.ensureReview(ctx, d, pending)
			if err != nil {
				return nil, err
			}
			if err := p.be.AddFileThread(ctx, id, filePath, in.body); err != nil {
				return nil, err
			}
			return func() {
				d.pendingReview = id
				p.loadDetail(d)
			}, nil
		},
	})
}

func (p *PRs) replyTo(d *prData, th github.Thread) {
	last := ""
	if n := len(th.Comments); n > 0 {
		last = th.Comments[n-1].Author
	}
	var checks []dialogCheck
	if !th.Resolved {
		checks = []dialogCheck{{label: resolveAfterLabel}}
	}
	p.openDialog(dialog{
		title:       "Reply to " + last,
		placeholder: "Write a reply",
		action:      "Reply",
		checks:      checks,
		validate:    requireBody("Write a reply first."),
		submit: func(ctx context.Context, in dialogInput) (func(), error) {
			comment, err := p.be.Reply(ctx, th.ID, in.body)
			if err != nil {
				return nil, err
			}
			resolve := len(in.checks) > 0 && in.checks[0]
			var resolveErr error
			if resolve {
				resolveErr = p.be.ResolveThread(ctx, th.ID)
			}
			return func() {
				for i := range d.threads {
					if d.threads[i].ID == th.ID {
						d.threads[i].Comments = append(d.threads[i].Comments, comment)
						d.threads[i].Resolved = d.threads[i].Resolved || resolve && resolveErr == nil
					}
				}
				if resolveErr != nil {
					p.h.ShowError("The reply was sent, but the thread was not resolved", resolveErr)
				}
			}, nil
		},
	})
}

func (p *PRs) editThreadComment(d *prData, th github.Thread, cm github.Comment) {
	p.openDialog(dialog{
		title:       "Edit Comment",
		placeholder: "Comment",
		action:      "Save",
		body:        cm.Body,
		validate:    requireBody("The comment cannot be empty."),
		submit: func(ctx context.Context, in dialogInput) (func(), error) {
			if err := p.be.EditComment(ctx, commentReview, cm.ID, in.body); err != nil {
				return nil, err
			}
			return func() {
				for i := range d.threads {
					for j := range d.threads[i].Comments {
						if d.threads[i].Comments[j].ID == cm.ID {
							d.threads[i].Comments[j].Body = in.body
						}
					}
				}
			}, nil
		},
	})
}

func (p *PRs) deleteThreadComment(d *prData, cm github.Comment) {
	p.confirmThen("Delete this comment?", "This cannot be undone.", "Delete", func() {
		be := p.be
		p.act(&d.busy, "Deleting comment", "Cannot delete the comment", func(ctx context.Context) (func(), error) {
			if err := be.DeleteComment(ctx, commentReview, cm.ID); err != nil {
				return nil, err
			}
			return func() {
				for i := range d.threads {
					d.threads[i].Comments = slices.DeleteFunc(d.threads[i].Comments, func(x github.Comment) bool { return x.ID == cm.ID })
				}
				d.threads = slices.DeleteFunc(d.threads, func(t github.Thread) bool { return len(t.Comments) == 0 })
			}, nil
		})
	})
}

func (p *PRs) setResolved(d *prData, th github.Thread, resolved bool) {
	label, failure := "Resolving thread", "Cannot resolve thread"
	if !resolved {
		label, failure = "Reopening thread", "Cannot unresolve thread"
	}
	be := p.be
	p.act(&d.busy, label, failure, func(ctx context.Context) (func(), error) {
		var err error
		if resolved {
			err = be.ResolveThread(ctx, th.ID)
		} else {
			err = be.UnresolveThread(ctx, th.ID)
		}
		if err != nil {
			return nil, err
		}
		return func() {
			for i := range d.threads {
				if d.threads[i].ID == th.ID {
					d.threads[i].Resolved = resolved
				}
			}
		}, nil
	})
}

func (p *PRs) applySuggestion(d *prData, th github.Thread, code string) {
	root := p.h.RootDir()
	target := filepath.Join(root, filepath.FromSlash(th.Path))
	switch {
	case p.branch.OID == "" || p.branch.OID != d.pr.HeadSHA:
		p.h.Notify("Check out this pull request at its latest commit to apply the suggestion.")
		return
	case !fsutil.Within(root, target):
		p.h.Notify("The suggestion is outside the project folder.")
		return
	}
	if buf := p.h.Buffer(target); buf != nil && buf.Modified() {
		p.h.Notify("Save or revert " + th.Path + " first, then apply the suggestion.")
		return
	}
	start, end := th.StartLine, th.Line
	if start == 0 {
		start = end
	}
	p.act(&d.busy, "Applying suggestion", "Cannot apply the suggestion", func(context.Context) (func(), error) {
		info, err := os.Stat(target)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(target)
		if err != nil {
			return nil, err
		}
		text, err := applySuggestionText(string(data), start, end, code)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, []byte(text), info.Mode().Perm()); err != nil {
			return nil, err
		}
		return func() { p.h.Notify("Applied the suggestion to " + th.Path) }, nil
	})
}

func (p *PRs) diffFiles(t *diffTab) []github.PRFile {
	if t.spec.mode == modeSince {
		return t.data.since
	}
	return t.data.files
}

func (p *PRs) stepFile(t *diffTab, delta int) {
	files := p.diffFiles(t)
	i := slices.IndexFunc(files, func(f github.PRFile) bool { return f.Path == t.file.Path })
	if i < 0 || i+delta < 0 || i+delta >= len(files) {
		return
	}
	p.openDiff(t.data, files[i+delta], t.spec)
}

func (t *diffTab) lineRange() (int, int) {
	if t.state == nil {
		return 0, 0
	}
	r, ok := t.state.SelectedLines()
	if !ok {
		return 0, 0
	}
	return r.Start + 1, r.End + 1
}

func (p *PRs) linkTarget(t *diffTab) (github.Remote, string) {
	repo := p.repo
	sha := t.data.pr.HeadSHA
	if r, ok := t.state.SelectedLines(); ok && r.Side == editor.DiffOld && t.oldSHA != "" {
		sha = t.oldSHA
	}
	return repo, sha
}

func (p *PRs) linkMenu(c *ui.Context, m *ui.Menu, t *diffTab) {
	d := t.data
	start, end := t.lineRange()
	repo, sha := p.linkTarget(t)
	permalink := blobURL(repo, sha, t.file.Path, start, end)
	headRepo := repo
	if d.pr.HeadOwner != "" && d.pr.HeadRepo != "" {
		headRepo = github.Remote{Host: repo.Host, Owner: d.pr.HeadOwner, Name: d.pr.HeadRepo}
	}
	if m.Item("Copy GitHub Permalink").Chosen() {
		c.WriteClipboard(permalink)
	}
	if m.Item("Copy Link to Branch").Disabled(d.pr.HeadRef == "").Chosen() {
		c.WriteClipboard(blobURL(headRepo, d.pr.HeadRef, t.file.Path, start, end))
	}
	if m.Item("Copy as Markdown").Chosen() {
		c.WriteClipboard(markdownPermalink(t.file.Path, start, end, permalink))
	}
	if m.Item("Copy Pull Request Link").Disabled(d.pr.URL == "").Chosen() {
		c.WriteClipboard(d.pr.URL)
	}
	m.Separator()
	if m.Item("Open Permalink on GitHub").Chosen() {
		openExternal(c, permalink)
	}
	dev, ok := devURL(repo, sha, t.file.Path, start, end)
	if m.Item("Open in github.dev").Disabled(!ok).Chosen() {
		openExternal(c, dev)
	}
}

func (p *PRs) diffView(c *ui.Context, t *diffTab) {
	if p.closed {
		return
	}
	th := c.Theme()
	d := t.data
	ui.Column(c).Fill().Background(th.Background).Children(func() {
		p.diffHeader(c, t)
		switch {
		case t.err != nil:
			ui.Column(c).Grow(1).FillWidth().Center().Gap(spacingMedium).Padding(messagePad).Children(func() {
				ui.Text(c, "Cannot show "+t.file.Path).Bold()
				ui.Text(c, errorText(t.err)).FontSize(fontSmall).TextColor(th.TextMuted).TextAlign(ui.Center)
				if !errors.Is(t.err, ErrBinaryFile) && !errors.Is(t.err, ErrFileTooLarge) {
					ui.Button(c, "Retry").OnClick(func() { p.loadDiff(t) })
				}
			})
		case !t.ready || !t.engine.Ready():
			ui.Column(c).Grow(1).FillWidth().Center().Children(func() { loadingRow(c, "Loading "+path.Base(t.file.Path)+"...") })
		default:
			st := t.state
			st.Layout = editor.DiffSplit
			if t.layout == layoutInline {
				st.Layout = editor.DiffUnified
			}
			t.engine.Refresh()
			var threads []editor.DiffAnnotation
			if t.spec.mode == modePR && !t.hideComments {
				threads = buildAnnotations(d.threads, t.file.Path, t.expanded, st.Old.LineCount(), st.New.LineCount())
			}
			st.Annotations = t.engine.Annotations(threads)
			editor.DiffView(c, st).Grow(1).FillWidth().Label("Diff of " + t.file.Path)
		}
	})
}

func (p *PRs) diffHeader(c *ui.Context, t *diffTab) {
	th := c.Theme()
	d := t.data
	ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Wrap().Padding(spacingMedium, spacingLarge).Shrink(0).FillWidth().Background(th.Surface).BorderWidth(0, 0, 1, 0).BorderColor(th.Border).Children(func() {
		ui.Text(c, t.file.Path).FontSize(fontBody).Bold().SingleLine().Shrink(1).MinWidth(0)
		ui.Text(c, "+"+strconv.Itoa(t.file.Additions)).FontSize(fontSmall).TextColor(th.Success).Shrink(0)
		ui.Text(c, "-"+strconv.Itoa(t.file.Deletions)).FontSize(fontSmall).TextColor(th.Danger).Shrink(0)
		if n := len(threadsForPath(d.threads, t.file.Path)); n > 0 && t.spec.mode == modePR {
			ui.Icon(c, icons.Comment).Size(iconSmall, iconSmall).TextColor(th.TextMuted).Shrink(0).Label(fmt.Sprintf("%d threads", n))
			mutedText(c, strconv.Itoa(n)).Shrink(0)
		}
		ui.Box(c).Grow(1)
		if t.loading || d.loading {
			ui.Spinner(c).Label("Loading")
		}
		if t.ready {
			p.diffControls(c, t)
		}
		ui.Button(c, "Open Pull Request").OnClick(func() { p.openPR(d.number, nil) })
	})
}

func (p *PRs) diffControls(c *ui.Context, t *diffTab) {
	d := t.data
	st := t.state
	if t.spec.mode == modePR && d.pr.NodeID != "" {
		viewed := d.viewed[t.file.Path]
		if ui.Checkbox(c.Key("viewed"), &viewed, "Viewed").Changed() {
			p.setViewed(d, t.file.Path, viewed)
		}
	}
	if t.spec.mode == modePR {
		ui.Toggle(c.Key("hide-comments"), &t.hideComments, "Hide Comments")
		ui.Button(c, "Comment on File").Disabled(!d.isOpen()).OnClick(func() { p.commentOnFile(t) })
		ui.Button(c, "Compare with Local").OnClick(func() { p.openDiff(d, t.file, diffSpec{mode: modeLocal}) })
	}
	ui.MenuButton(c, "Link", func(m *ui.Menu) { p.linkMenu(c, m, t) }).Label("Copy or open a link")
	files := p.diffFiles(t)
	i := slices.IndexFunc(files, func(f github.PRFile) bool { return f.Path == t.file.Path })
	ui.Button(c, "Previous File").Disabled(i <= 0).OnClick(func() { p.stepFile(t, -1) })
	ui.Button(c, "Next File").Disabled(i < 0 || i+1 >= len(files)).OnClick(func() { p.stepFile(t, 1) })
	ui.Segmented(c, &t.layout, "Split", "Unified").Label("Diff layout")
	diffmode.Controls(c, t.engine)
	h, _ := st.CurrentHunk()
	n := len(st.Hunks())
	ui.Button(c, "Previous Change").Disabled(n == 0 || h <= 0).OnClick(func() { st.GoToHunk(h - 1) })
	ui.Button(c, "Next Change").Disabled(n == 0 || h+1 >= n).OnClick(func() { st.GoToHunk(h + 1) })
}

func (p *PRs) threadView(c *ui.Context, t *diffTab, a editor.DiffAnnotation) {
	id, _ := a.Key.(string)
	d := t.data
	var th *github.Thread
	for i := range d.threads {
		if d.threads[i].ID == id {
			th = &d.threads[i]
		}
	}
	if th == nil {
		return
	}
	theme := c.Theme()
	now := p.env.Now()
	box := ui.Column(c.Key(id)).FillWidth().Height(a.Height).ClipY().Padding(threadPadY, threadPadX).Gap(threadGap).Background(theme.Surface).Border(1, theme.Border).Radius(cardRadius)
	thread := *th
	box.Children(func() {
		collapsed := thread.Resolved && !t.expanded[id]
		ui.Row(c).Gap(spacingSmall).AlignItems(ui.Center).FillWidth().Children(func() {
			if thread.Resolved {
				pill(c, "Resolved", theme.Success)
			}
			if thread.Outdated {
				mutedPill(c, "Outdated")
			}
			if collapsed {
				mutedText(c, fmt.Sprintf("%d comments", len(thread.Comments))).Grow(1)
				ui.Button(c, "Show").Label("Show thread").OnClick(func() { t.expanded[id] = true })
				return
			}
			ui.Box(c).Grow(1)
			if thread.Resolved {
				ui.Button(c, "Hide").Label("Hide thread").OnClick(func() { delete(t.expanded, id) })
			}
		})
		if collapsed {
			return
		}
		for i, cm := range thread.Comments {
			p.threadComment(c, d, thread, i, cm, now)
		}
		ui.Row(c).Gap(spacingSmall).FillWidth().Children(func() {
			ui.Button(c, "Reply").Disabled(d.busy != "").OnClick(func() { p.replyTo(d, thread) })
			if thread.Resolved {
				ui.Button(c, "Unresolve").Disabled(d.busy != "").OnClick(func() { p.setResolved(d, thread, false) })
			} else {
				ui.Button(c, "Resolve").Disabled(d.busy != "").OnClick(func() { p.setResolved(d, thread, true) })
			}
		})
	})
}

func (p *PRs) threadComment(c *ui.Context, d *prData, th github.Thread, i int, cm github.Comment, now time.Time) {
	own := p.viewer != "" && cm.Author == p.viewer
	suggestion, hasSuggestion := suggestionOf(cm.Body)
	ui.Column(c.Key(i)).Gap(2).FillWidth().Children(func() {
		ui.Row(c).Gap(spacingSmall).AlignItems(ui.Center).FillWidth().Children(func() {
			ui.Text(c, cm.Author).FontSize(fontSmall).Bold()
			mutedText(c, relativeTime(now, cm.Created))
			ui.Box(c).Grow(1)
			if cm.URL != "" {
				ui.Button(c, "Copy Link").Label("Copy link to comment by " + cm.Author).OnClick(func() { c.WriteClipboard(cm.URL) })
			}
			if hasSuggestion && th.Side == github.SideRight && !th.Outdated {
				ui.Button(c, "Apply Suggestion").Disabled(d.busy != "").OnClick(func() { p.applySuggestion(d, th, suggestion) })
			}
			if own {
				ui.Button(c, "Edit").Label("Edit comment by " + cm.Author).Disabled(d.busy != "").OnClick(func() { p.editThreadComment(d, th, cm) })
				ui.Button(c, "Delete").Label("Delete comment by " + cm.Author).Disabled(d.busy != "").OnClick(func() { p.deleteThreadComment(d, cm) })
			}
		})
		ui.Text(c, cm.Body).FontSize(fontBody).Selectable()
	})
}
