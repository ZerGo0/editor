package gitui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ZerGo0/editor/internal/git"
)

const (
	overwriteMarker   = "would be overwritten"
	notMergedMarker   = "not fully merged"
	defaultRemoteName = "origin"
	headsPrefix       = "refs/heads/"
	tagsPrefix        = "refs/tags/"
)

var errBranchName = errors.New("Enter a branch name without spaces.")

type branchAction struct {
	label     string
	danger    bool
	disabled  bool
	separator bool
	run       func()
}

func quoted(s string) string { return "'" + s + "'" }

func (g *Git) branchActions(e branchEntry) []branchAction {
	cur := g.currentBranch()
	curLabel := quoted(g.headLabel())
	name := quoted(e.name)
	busy := g.busy != ""
	sep := branchAction{separator: true}
	var acts []branchAction
	add := func(a ...branchAction) { acts = append(acts, a...) }
	if e.branch.Current {
		add(
			branchAction{label: "New Branch from " + name + "...", run: func() { g.newBranch(e.name) }},
			branchAction{label: "Push...", disabled: busy, run: g.openPush},
			branchAction{label: "Update", disabled: busy || e.branch.Upstream == "", run: g.pull},
			sep,
			branchAction{label: "Rename...", run: func() { g.renameBranch(e.name) }},
		)
		return acts
	}
	checkout := branchAction{label: "Checkout", disabled: busy, run: func() { g.checkout(e.name, e.name) }}
	if e.kind == kindRemote {
		checkout.run = func() { g.checkoutRemote(e.name) }
	}
	add(checkout, branchAction{label: "New Branch from " + name + "...", run: func() { g.newBranch(e.name) }})
	if e.kind == kindLocal && cur != "" {
		add(branchAction{label: "Checkout and Rebase onto " + curLabel, disabled: busy, run: func() { g.checkoutAndRebase(e.name, cur) }})
	}
	add(
		sep,
		branchAction{label: "Compare with " + curLabel, disabled: cur == "" && g.snap.status == nil, run: func() { g.compareBranches(headRev, e.name) }},
		branchAction{label: "Show Diff with Working Tree", run: func() { g.compareWithLocal(e.name) }},
		sep,
	)
	if e.kind != kindTag {
		add(branchAction{label: "Rebase " + curLabel + " onto " + name, disabled: busy, run: func() { g.rebaseOnto(e.name) }})
	}
	add(branchAction{label: "Merge " + name + " into " + curLabel, disabled: busy, run: func() { g.mergeInto(e.name) }})
	if e.kind != kindTag {
		add(branchAction{label: "New Worktree from " + name + "...", run: func() { g.openNewWorktree(e.name) }})
	}
	add(sep)
	switch e.kind {
	case kindLocal:
		add(
			branchAction{label: "Push...", disabled: busy, run: func() { g.pushBranch(e.branch) }},
			branchAction{label: "Rename...", run: func() { g.renameBranch(e.name) }},
			branchAction{label: "Delete", danger: true, disabled: busy, run: func() { g.deleteBranch(e.name) }},
		)
	case kindRemote:
		add(branchAction{label: "Delete", danger: true, disabled: busy, run: func() { g.deleteRemoteBranch(e.name) }})
	case kindTag:
		add(
			branchAction{label: "Push to " + quoted(g.defaultRemote()), disabled: busy || g.defaultRemote() == "", run: func() { g.pushTag(e.name) }},
			branchAction{label: "Delete", danger: true, disabled: busy, run: func() { g.deleteTag(e.name) }},
			branchAction{label: "Delete on Remote", danger: true, disabled: busy || g.defaultRemote() == "", run: func() { g.deleteRemoteTag(e.name) }},
		)
	}
	return acts
}

func (g *Git) defaultRemote() string {
	if st := g.snap.status; st != nil && st.Upstream != "" {
		if remote, _, ok := strings.Cut(st.Upstream, "/"); ok {
			return remote
		}
	}
	for _, r := range g.snap.remotes {
		if r.Name == defaultRemoteName {
			return r.Name
		}
	}
	if len(g.snap.remotes) > 0 {
		return g.snap.remotes[0].Name
	}
	return ""
}

func (g *Git) checkout(ref, label string) {
	dirty := false
	g.do(op{
		title:  "Checkout",
		banner: true,
		run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
			err := r.Checkout(ctx, ref, false)
			if err != nil && strings.Contains(err.Error(), overwriteMarker) {
				dirty = true
				return git.OpResult{}, nil
			}
			return git.OpResult{}, err
		},
		done: func(git.OpResult) {
			if dirty {
				g.offerSmartCheckout(ref, label)
				return
			}
			g.h.Notify("Checked out " + label)
			g.headChanged()
		},
	})
}

func (g *Git) offerSmartCheckout(ref, label string) {
	g.confirm(question{
		message: "Your local changes would be overwritten by the checkout of " + label + ".",
		detail:  "Smart Checkout stashes the changes, checks out " + label + ", and applies the stash again.",
		action:  "Smart Checkout",
	}, func() {
		g.do(op{
			title:  "Smart Checkout",
			banner: true,
			run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
				saved, err := r.StashPush(ctx, "Smart checkout to "+label, true, false)
				if err != nil {
					return git.OpResult{}, err
				}
				if err := r.Checkout(ctx, ref, false); err != nil {
					if saved {
						_, _ = r.StashApply(ctx, 0, true)
					}
					return git.OpResult{}, err
				}
				if !saved {
					return git.OpResult{}, nil
				}
				return r.StashApply(ctx, 0, true)
			},
			done: func(git.OpResult) {
				g.h.Notify("Checked out " + label)
				g.headChanged()
			},
		})
	})
}

func (g *Git) checkoutRemote(name string) {
	g.do(op{
		title:  "Checkout",
		banner: true,
		run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
			return git.OpResult{}, r.CheckoutNewTracking(ctx, name)
		},
		done: func(git.OpResult) {
			g.h.Notify("Checked out " + name)
			g.headChanged()
		},
	})
}

func (g *Git) checkoutRevision() {
	g.h.Ask("Checkout Tag or Revision", "Checkout", "", func(rev string) error {
		rev = strings.TrimSpace(rev)
		if err := checkName(rev); err != nil {
			return err
		}
		g.checkout(rev, rev)
		return nil
	})
}

func (g *Git) checkoutAndRebase(name, onto string) {
	g.do(op{
		title:  "Rebase",
		banner: true,
		run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
			if err := r.Checkout(ctx, name, false); err != nil {
				return git.OpResult{}, err
			}
			return r.Rebase(ctx, onto)
		},
		done: func(git.OpResult) { g.headChanged() },
	})
}

func (g *Git) rebaseOnto(ref string) {
	g.do(op{
		title:  "Rebase",
		banner: true,
		run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
			return r.Rebase(ctx, ref)
		},
		done: func(res git.OpResult) {
			if !res.Stopped {
				g.h.Notify("Rebased " + g.headLabel() + " onto " + ref)
			}
			g.headChanged()
		},
	})
}

func (g *Git) mergeInto(ref string) {
	g.do(op{
		title:  "Merge",
		banner: true,
		run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
			return r.Merge(ctx, ref, git.MergeOptions{})
		},
		done: func(res git.OpResult) {
			if !res.Stopped {
				g.h.Notify("Merged " + ref + " into " + g.headLabel())
			}
			g.headChanged()
		},
	})
}

func (g *Git) renameBranch(name string) {
	g.h.Ask("Rename Branch", "Rename", name, func(next string) error {
		next = strings.TrimSpace(next)
		if err := g.validBranchName(next); err != nil {
			return err
		}
		g.doSimple("Rename Branch", func(ctx context.Context, r repo) error { return r.RenameBranch(ctx, name, next) }, nil)
		return nil
	})
}

func (g *Git) validBranchName(name string) error {
	if err := checkName(name); err != nil {
		return errBranchName
	}
	if strings.Contains(name, "..") || strings.HasSuffix(name, "/") || strings.HasSuffix(name, ".lock") {
		return fmt.Errorf("%q is not a valid branch name.", name)
	}
	for _, b := range g.snap.branches {
		if !b.Remote && b.Name == name {
			return fmt.Errorf("A branch named %q already exists.", name)
		}
	}
	return nil
}

func (g *Git) deleteBranch(name string) {
	g.confirm(question{
		message:     "Delete branch " + quoted(name) + "?",
		action:      "Delete",
		destructive: true,
	}, func() {
		unmerged := false
		g.do(op{
			title: "Delete Branch",
			run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
				err := r.DeleteBranch(ctx, name, false)
				if err != nil && strings.Contains(err.Error(), notMergedMarker) {
					unmerged = true
					return git.OpResult{}, nil
				}
				return git.OpResult{}, err
			},
			done: func(git.OpResult) {
				if !unmerged {
					g.h.Notify("Deleted branch " + name)
					return
				}
				g.confirm(question{
					message:     "Branch " + quoted(name) + " is not fully merged.",
					detail:      "Its unmerged commits can be lost.",
					action:      "Force Delete",
					destructive: true,
				}, func() {
					g.doSimple("Delete Branch", func(ctx context.Context, r repo) error { return r.DeleteBranch(ctx, name, true) },
						func() { g.h.Notify("Deleted branch " + name) })
				})
			},
		})
	})
}

func (g *Git) deleteRemoteBranch(name string) {
	remote, branch, ok := strings.Cut(name, "/")
	if !ok {
		return
	}
	g.confirm(question{
		message:     "Delete remote branch " + quoted(name) + "?",
		detail:      "The branch is deleted on " + remote + ".",
		action:      "Delete",
		destructive: true,
	}, func() {
		g.remoteRef("Delete Remote Branch", remote, ":"+headsPrefix+branch, "Deleted "+name)
	})
}

func (g *Git) remoteRef(title, remote, refspec, notice string) {
	g.do(op{
		title:   title,
		banner:  true,
		network: true,
		run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
			return git.OpResult{}, r.PushRef(ctx, remote, refspec)
		},
		done: func(git.OpResult) { g.h.Notify(notice) },
	})
}

func (g *Git) pushBranch(b git.Branch) {
	remote := g.defaultRemote()
	if remote == "" {
		g.openPush()
		return
	}
	g.do(op{
		title:   "Push",
		banner:  true,
		network: true,
		run: func(ctx context.Context, r repo, progress func(string)) (git.OpResult, error) {
			return git.OpResult{}, r.Push(ctx, git.PushOptions{Remote: remote, Branch: b.Name, SetUpstream: b.Upstream == "", Progress: progress})
		},
		done: func(git.OpResult) { g.h.Notify("Pushed " + b.Name + " to " + remote) },
	})
}

func (g *Git) pushTag(name string) {
	remote := g.defaultRemote()
	g.remoteRef("Push Tag", remote, tagsPrefix+name, "Pushed tag "+name+" to "+remote)
}

func (g *Git) deleteTag(name string) {
	g.confirm(question{message: "Delete tag " + quoted(name) + "?", action: "Delete", destructive: true}, func() {
		g.doSimple("Delete Tag", func(ctx context.Context, r repo) error { return r.DeleteTag(ctx, name) }, func() {
			g.h.Notify("Deleted tag " + name)
			g.loadTags()
		})
	})
}

func (g *Git) deleteRemoteTag(name string) {
	remote := g.defaultRemote()
	g.confirm(question{
		message:     "Delete tag " + quoted(name) + " on " + remote + "?",
		action:      "Delete",
		destructive: true,
	}, func() {
		g.remoteRef("Delete Remote Tag", remote, ":"+tagsPrefix+name, "Deleted tag "+name+" on "+remote)
	})
}
