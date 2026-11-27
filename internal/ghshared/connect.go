package ghshared

import (
	"context"
	"errors"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
)

type Phase int

const (
	PhaseLoading Phase = iota
	PhaseNoRepo
	PhaseNoRemote
	PhaseNoGH
	PhaseNotLoggedIn
	PhaseError
	PhaseReady
)

type RemoteLister interface {
	Remotes(ctx context.Context) ([]git.Remote, error)
}

type Resolution[L RemoteLister] struct {
	Phase   Phase
	Err     error
	Local   L
	Repo    github.Remote
	Remotes map[string]github.Remote
	Token   string
}

func LocalOpener[L any](wrap func(*git.Repo) L) func(ctx context.Context, dir string) (L, error) {
	return func(ctx context.Context, dir string) (L, error) {
		repo, err := git.Open(ctx, dir)
		if err != nil {
			var none L
			return none, err
		}
		return wrap(repo), nil
	}
}

func Resolve[L RemoteLister](ctx context.Context, open func(context.Context, string) (L, error), token func(context.Context, string) (string, error), root string) Resolution[L] {
	local, err := open(ctx, root)
	if err != nil {
		if errors.Is(err, git.ErrNotRepo) {
			return Resolution[L]{Phase: PhaseNoRepo}
		}
		return Resolution[L]{Phase: PhaseError, Err: err}
	}
	remotes, err := local.Remotes(ctx)
	if err != nil {
		return Resolution[L]{Phase: PhaseError, Err: err}
	}
	repo, byName, ok := PickRemote(remotes)
	if !ok {
		return Resolution[L]{Phase: PhaseNoRemote}
	}
	tok, err := token(ctx, repo.Host)
	switch {
	case errors.Is(err, github.ErrNoGH):
		return Resolution[L]{Phase: PhaseNoGH, Repo: repo}
	case errors.Is(err, github.ErrNotLoggedIn):
		return Resolution[L]{Phase: PhaseNotLoggedIn, Repo: repo, Err: err}
	case err != nil:
		return Resolution[L]{Phase: PhaseError, Repo: repo, Err: err}
	}
	return Resolution[L]{Phase: PhaseReady, Local: local, Repo: repo, Remotes: byName, Token: tok}
}
