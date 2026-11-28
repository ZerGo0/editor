package gitui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/git"
)

const pushUpToErrorTitle = "Cannot push up to this commit"

type resetMode struct {
	mode   git.ResetMode
	label  string
	detail string
	danger bool
}

var resetModes = []resetMode{
	{mode: git.ResetSoft, label: "Soft", detail: "Keep all changes staged"},
	{mode: git.ResetMixed, label: "Mixed", detail: "Keep all changes, unstaged"},
	{mode: git.ResetHard, label: "Hard", detail: "Discard all local changes", danger: true},
	{mode: git.ResetKeep, label: "Keep", detail: "Keep local changes, discard commits"},
}

func (g *Git) resetTo(oid string, m resetMode) {
	q := question{
		message:     fmt.Sprintf("Reset %s to %s (%s)?", g.headLabel(), shortHash(oid), m.label),
		detail:      m.detail + ".",
		action:      "Reset",
		destructive: m.danger,
	}
	g.confirm(q, func() {
		g.doSimple("Reset", func(ctx context.Context, r repo) error { return r.Reset(ctx, oid, m.mode) }, func() {
			g.headChanged()
			g.reloadAllBuffers()
		})
	})
}

func (g *Git) reloadAllBuffers() {
	if g.snap.status == nil {
		return
	}
	paths := make([]string, 0, len(g.snap.status.Files))
	for _, f := range g.snap.status.Files {
		paths = append(paths, f.Path)
	}
	g.reloadBuffers(paths)
}

func (g *Git) cherryPick(cm git.Commit) {
	g.do(op{
		title:  "Cherry-Pick",
		banner: true,
		run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
			return r.CherryPick(ctx, cm.OID)
		},
		done: func(res git.OpResult) {
			if !res.Stopped {
				g.h.Notify("Cherry-picked " + shortHash(cm.OID))
			}
			g.headChanged()
		},
	})
}

func (g *Git) revertCommit(cm git.Commit) {
	g.do(op{
		title:  "Revert",
		banner: true,
		run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
			return r.Revert(ctx, cm.OID)
		},
		done: func(res git.OpResult) {
			if !res.Stopped {
				g.h.Notify("Reverted " + shortHash(cm.OID))
			}
			g.headChanged()
		},
	})
}

func (g *Git) undoCommit(cm git.Commit) {
	parent := cm.Parents[0]
	g.doSimple("Undo Commit", func(ctx context.Context, r repo) error {
		return r.Reset(ctx, parent, git.ResetSoft)
	}, func() {
		if strings.TrimSpace(g.commit.message) == "" {
			g.commit.message = fullMessage(cm)
		}
		g.headChanged()
		g.focusCommit()
	})
}

func fullMessage(cm git.Commit) string {
	if body := strings.TrimSpace(cm.Body); body != "" {
		return cm.Subject + "\n\n" + body
	}
	return cm.Subject
}

func (g *Git) rewordCommit(cm git.Commit) {
	g.h.Ask("Edit Commit Message", "Reword", cm.Subject, func(subject string) error {
		subject = strings.TrimSpace(subject)
		if subject == "" {
			return git.ErrEmptyMessage
		}
		msg := subject
		if body := strings.TrimSpace(cm.Body); body != "" {
			msg += "\n\n" + body
		}
		g.rewriteCommit("Edit Commit Message", cm, git.ActionReword, msg)
		return nil
	})
}

func (g *Git) dropCommit(cm git.Commit) {
	g.confirm(question{
		message:     "Drop commit " + shortHash(cm.OID) + "?",
		detail:      cm.Subject + "\n\nThe commit and its changes are removed from the branch history.",
		action:      "Drop",
		destructive: true,
	}, func() { g.rewriteCommit("Drop Commit", cm, git.ActionDrop, "") })
}

func setTodoAction(items []git.TodoItem, oid string, action git.TodoAction) (string, bool) {
	for i, it := range items {
		if it.OID != "" && (strings.HasPrefix(oid, it.OID) || strings.HasPrefix(it.OID, oid)) {
			items[i].Action = action
			return it.OID, true
		}
	}
	return "", false
}

func (g *Git) rewriteCommit(title string, cm git.Commit, action git.TodoAction, message string) {
	parent := cm.Parents[0]
	g.do(op{
		title:  title,
		banner: true,
		run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
			items, err := r.RebaseTodo(ctx, parent)
			if err != nil {
				return git.OpResult{}, err
			}
			key, ok := setTodoAction(items, cm.OID, action)
			if !ok {
				return git.OpResult{}, fmt.Errorf("commit %s is not in the current branch", shortHash(cm.OID))
			}
			var messages map[string]string
			if message != "" {
				messages = map[string]string{key: message}
			}
			return r.RebaseInteractive(ctx, parent, items, messages)
		},
		done: func(git.OpResult) { g.headChanged() },
	})
}

func (g *Git) pushUpTo(cm git.Commit) {
	st := g.snap.status
	if st == nil || st.Upstream == "" {
		g.h.ShowError(pushUpToErrorTitle, errors.New("The current branch has no upstream. Push it once from the Push dialog."))
		return
	}
	remote, branch, ok := strings.Cut(st.Upstream, "/")
	if !ok {
		g.h.ShowError(pushUpToErrorTitle, errors.New("Cannot read the upstream branch "+st.Upstream+"."))
		return
	}
	refspec := cm.OID + ":refs/heads/" + branch
	g.confirm(question{
		message: fmt.Sprintf("Push all commits up to %s to %s?", shortHash(cm.OID), st.Upstream),
		action:  "Push",
	}, func() {
		g.do(op{
			title:   "Push",
			banner:  true,
			network: true,
			run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
				return git.OpResult{}, r.PushRef(ctx, remote, refspec)
			},
			done: func(git.OpResult) { g.h.Notify("Pushed up to " + shortHash(cm.OID)) },
		})
	})
}

func (g *Git) copyCommitPatch(cm git.Commit) {
	if !g.ready() {
		return
	}
	r := g.repo
	applog.Go(func() {
		var patch []byte
		fds, err := diffOfCommit(g.ctx, r, cm)
		if err == nil {
			patch = joinPatches(fds)
		}
		g.post(func() {
			if err != nil {
				g.h.ShowError("Cannot create the patch", err)
				return
			}
			g.clipboard = string(patch)
			g.h.Notify("Copied the patch of " + shortHash(cm.OID))
		})
	})
}

const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

func diffOfCommit(ctx context.Context, r repo, cm git.Commit) ([]git.FileDiff, error) {
	parent := emptyTree
	if len(cm.Parents) > 0 {
		parent = cm.Parents[0]
	}
	return r.DiffCommits(ctx, parent, cm.OID)
}

func joinPatches(fds []git.FileDiff) []byte {
	var b bytes.Buffer
	for _, fd := range fds {
		b.Write(git.BuildPatch(fd, func(int, int) bool { return true }, false))
	}
	return b.Bytes()
}
