package gitui

import (
	"context"

	"github.com/ZerGo0/editor/internal/git"
)

func (g *Git) fetch() {
	g.do(op{
		title:   "Fetch",
		banner:  true,
		network: true,
		run: func(ctx context.Context, r repo, progress func(string)) (git.OpResult, error) {
			return git.OpResult{}, r.Fetch(ctx, "", true, progress)
		},
		done: func(git.OpResult) { g.h.Notify("Fetched all remotes") },
	})
}

func (g *Git) pull() {
	g.pullWith(g.pullOptions())
}

func (g *Git) pullWith(o git.PullOptions) {
	g.do(op{
		title:  "Pull",
		banner: true,
		run: func(ctx context.Context, r repo, progress func(string)) (git.OpResult, error) {
			o.Progress = progress
			return r.Pull(ctx, o)
		},
		done: func(res git.OpResult) {
			if !res.Stopped {
				g.h.Notify("Pulled " + g.headLabel())
			}
			g.headChanged()
		},
	})
}

func (g *Git) continueOp() {
	g.do(op{
		title:  "Continue",
		banner: true,
		run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
			return r.Continue(ctx)
		},
		done: func(git.OpResult) { g.headChanged() },
	})
}

func (g *Git) skipOp() {
	g.do(op{
		title:  "Skip",
		banner: true,
		run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
			return r.Skip(ctx)
		},
		done: func(git.OpResult) { g.headChanged() },
	})
}

func (g *Git) abortOp() {
	g.confirm(question{
		message:     "Abort the " + g.snap.op.Kind.String() + "?",
		detail:      "The work tree goes back to where it was before the operation started.",
		action:      "Abort",
		destructive: true,
	}, func() {
		g.do(op{
			title:  "Abort",
			banner: true,
			run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
				return git.OpResult{}, r.Abort(ctx)
			},
			done: func(git.OpResult) { g.headChanged() },
		})
	})
}

func (g *Git) update() {
	g.do(op{
		title:  "Update Project",
		banner: true,
		run: func(ctx context.Context, r repo, progress func(string)) (git.OpResult, error) {
			if err := r.Fetch(ctx, "", true, progress); err != nil {
				return git.OpResult{}, err
			}
			o := g.pullOptions()
			o.Progress = progress
			return r.Pull(ctx, o)
		},
		done: func(res git.OpResult) {
			if !res.Stopped {
				g.h.Notify("Updated " + g.headLabel())
			}
			g.headChanged()
		},
	})
}

func (g *Git) pushUpstream() {
	g.startPush(git.PushOptions{}, func() { g.h.Notify("Pushed " + g.headLabel()) })
}

func (g *Git) pushOrOpen() {
	if st := g.snap.status; st != nil && st.Upstream != "" {
		g.pushUpstream()
		return
	}
	g.openPush()
}

func (g *Git) syncChanges() {
	g.do(op{
		title:  "Sync",
		banner: true,
		run: func(ctx context.Context, r repo, progress func(string)) (git.OpResult, error) {
			o := g.pullOptions()
			o.Progress = progress
			res, err := r.Pull(ctx, o)
			if err != nil || res.Stopped {
				return res, err
			}
			return res, r.Push(ctx, git.PushOptions{Progress: progress})
		},
		done: func(res git.OpResult) {
			if !res.Stopped {
				g.h.Notify("Synced " + g.headLabel())
			}
			g.headChanged()
		},
	})
}
