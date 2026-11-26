package gitui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	pushPreviewLimit = 100
	forcePushNote    = "Force push replaces the remote history. The lease stops it if the remote changed since the last fetch."
)

type pushState struct {
	open       bool
	loading    bool
	err        error
	gen        int
	commits    []git.Commit
	newBranch  bool
	list       ui.ListState
	selected   int
	files      []git.ChangedFile
	filesFor   string
	filesErr   error
	fileList   ui.ListState
	remote     string
	target     string
	force      bool
	tags       bool
	remoteName string
	remoteURL  string
}

func (g *Git) openPush() {
	if !g.ready() {
		return
	}
	s := &g.push
	*s = pushState{open: true, selected: -1, remote: g.defaultRemote(), remoteName: defaultRemoteName}
	s.target = g.currentBranch()
	if st := g.snap.status; st != nil && st.Upstream != "" {
		if _, branch, ok := strings.Cut(st.Upstream, "/"); ok {
			s.target = branch
		}
	}
	g.loadOutgoing()
}

func (g *Git) loadOutgoing() {
	s := &g.push
	s.gen++
	gen, r := s.gen, g.repo
	s.loading, s.err = true, nil
	go func() {
		commits, err := r.OutgoingCommits(g.ctx)
		newBranch := errors.Is(err, git.ErrNoUpstream)
		if newBranch {
			commits, err = r.Log(g.ctx, git.LogOptions{Revs: []string{headRev}, Max: pushPreviewLimit})
		}
		g.post(func() {
			if gen != s.gen {
				return
			}
			s.loading, s.err, s.commits, s.newBranch = false, err, commits, newBranch
			s.selected = -1
			if len(commits) > 0 {
				s.selected = 0
			}
		})
	}()
}

func (g *Git) loadPushFiles(oid string) {
	s := &g.push
	s.filesFor, s.files, s.filesErr = oid, nil, nil
	r := g.repo
	go func() {
		files, err := r.CommitFiles(g.ctx, oid)
		g.post(func() {
			if s.filesFor == oid {
				s.files, s.filesErr = files, err
			}
		})
	}()
}

func (g *Git) pushDialog(c *ui.Context) {
	s := &g.push
	if s.open && !g.ready() {
		s.open = false
	}
	modal(c, &s.open, "Push Commits", token.DialogWidthWide, func() {
		if len(g.snap.remotes) == 0 {
			g.defineRemote(c)
			return
		}
		g.pushTarget(c)
		g.pushPreview(c)
		ui.Row(c).Gap(token.GapGroup.Px()).Wrap().Children(func() {
			action.Checkbox(c, "Force push (with lease)", &s.force, action.ChoiceOptions{})
			action.Checkbox(c, "Push tags", &s.tags, action.ChoiceOptions{})
		})
		if s.force {
			feedback.Notice(c, feedback.NoticeOptions{Tone: token.ToneWarning, Text: forcePushNote})
		}
		target := strings.TrimSpace(s.target)
		action := "Push"
		if s.force {
			action = "Force Push"
		}
		disabled := g.busy != "" || target == "" || s.remote == "" || checkName(target) != nil
		dialogButtons(c, func() { s.open = false }, action, disabled, func() { g.runPush(target) })
	})
}

func (g *Git) pushTarget(c *ui.Context) {
	s := &g.push
	remotes := make([]action.Option[string], len(g.snap.remotes))
	for i, r := range g.snap.remotes {
		remotes[i] = action.Option[string]{Value: r.Name, Label: r.Name}
	}
	ui.Row(c).FillWidth().Gap(token.GapControl.Px()).AlignItems(ui.Center).Children(func() {
		token.TextStrong.Apply(ui.Text(c, g.headLabel())).SingleLine().Shrink(0)
		secondary(c, "\u2192").Shrink(0)
		action.Select(c, "Push Remote", &s.remote, remotes, action.SelectOptions{Width: token.FieldWidthSmall})
		secondary(c, ":").Shrink(0)
		ui.Row(c).Grow(1).MinWidth(0).Children(func() {
			field.TextInput(c, "Target Branch", &s.target, field.InputOptions{Placeholder: "Target branch", ID: "push-target"})
		})
	})
	if s.newBranch {
		secondary(c, "New branch: the branch has no upstream yet, so it is set on push.")
	}
}

func (g *Git) pushPreview(c *ui.Context) {
	s := &g.push
	switch {
	case s.err != nil:
		errorState(c, "Cannot load the outgoing commits", s.err, g.loadOutgoing)
		return
	case s.loading:
		loadingState(c, "Loading outgoing commits")
		return
	case len(s.commits) == 0:
		emptyState(c, "No commits to push", "You can still push tags or create the branch on the remote.")
		return
	}
	commits := s.commits
	if s.selected >= 0 && s.selected < len(commits) && s.filesFor != commits[s.selected].OID {
		g.loadPushFiles(commits[s.selected].OID)
	}
	ui.Row(c).FillWidth().Height(token.ListHeightMax.Px()).Gap(token.GapControl.Px()).Children(func() {
		s.list.Key = func(i int) any { return commits[i].OID }
		s.list.Label = func(i int) string { return commits[i].Subject }
		s.list.Selected = &s.selected
		pushColumn(c, func() {
			list.View(c.Key("push-commits"), &s.list, len(commits), list.ViewOptions{Label: "Outgoing Commits"}, func(i int) list.Item {
				cm := commits[i]
				return list.Item{Label: cm.Subject, Icon: icons.Commit, Description: shortHash(cm.OID),
					Tooltip: fmt.Sprintf("%s, %s", cm.Author, formatTime(cm.AuthorTime, c.Now()))}
			})
		})
		pushColumn(c, func() {
			switch {
			case s.filesErr != nil:
				errorState(c, "Cannot load the files", s.filesErr, func() { g.loadPushFiles(s.filesFor) })
			case s.files == nil:
				loadingState(c, "Loading files")
			default:
				files := s.files
				s.fileList.Label = func(i int) string { return files[i].Path }
				list.View(c.Key("push-files"), &s.fileList, len(files), list.ViewOptions{Label: "Commit Files"}, func(i int) list.Item {
					f := files[i]
					cm := commits[s.selected]
					return changedFileItem(f.Path, f.Status, func(preview, focus bool) { g.openDiffWith(commitSource(cm, f), preview, focus) })
				})
			}
		})
	})
}

func pushColumn(c *ui.Context, body func()) {
	p := token.Of(c)
	ui.Column(c).Grow(1).Basis(0).FillHeight().MinWidth(0).Radius(token.RadiusControl.Px()).Clip().
		Border(token.BorderWidth.Px(), p.Border.Input).Children(body)
}

func (g *Git) runPush(target string) {
	s := &g.push
	branch := g.currentBranch()
	refspec := target
	if branch == "" || branch != target {
		refspec = headRev + ":" + headsPrefix + target
	}
	opts := git.PushOptions{Remote: s.remote, Branch: refspec, Force: s.force, Tags: s.tags, SetUpstream: s.newBranch && branch != ""}
	run := func() {
		s.open = false
		g.do(op{
			title:   "Push",
			banner:  true,
			network: true,
			run: func(ctx context.Context, r repo, progress func(string)) (git.OpResult, error) {
				opts.Progress = progress
				return git.OpResult{}, r.Push(ctx, opts)
			},
			done: func(git.OpResult) { g.h.Notify("Pushed to " + opts.Remote + "/" + target) },
		})
	}
	if !s.force {
		run()
		return
	}
	g.confirm(question{
		message:     "Force push to " + s.remote + "/" + target + "?",
		detail:      "Commits on the remote that you do not have can be lost. The push uses --force-with-lease.",
		action:      "Force Push",
		destructive: true,
	}, run)
}

func (g *Git) defineRemote(c *ui.Context) {
	s := &g.push
	paragraph(c, "This repository has no remote. Define one to push.")
	field.TextInput(c, "Remote Name", &s.remoteName, field.InputOptions{Placeholder: "Name", ID: "remote-name"})
	field.TextInput(c, "Remote URL", &s.remoteURL, field.InputOptions{Placeholder: "URL", AutoFocus: true, ID: "remote-url"})
	name, url := strings.TrimSpace(s.remoteName), strings.TrimSpace(s.remoteURL)
	disabled := checkName(name) != nil || checkURL(url) != nil
	dialogButtons(c, func() { s.open = false }, "Define Remote", disabled, func() {
		g.doSimple("Define Remote", func(ctx context.Context, r repo) error { return r.AddRemote(ctx, name, url) }, func() {
			s.remote = name
			g.loadOutgoing()
		})
	})
}
