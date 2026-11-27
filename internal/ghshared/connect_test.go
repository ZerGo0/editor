package ghshared_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ZerGo0/editor/internal/ghshared"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
)

type fakeLister struct {
	remotes []git.Remote
	err     error
}

func (f fakeLister) Remotes(context.Context) ([]git.Remote, error) { return f.remotes, f.err }

func TestResolvePhases(t *testing.T) {
	good := []git.Remote{{Name: "origin", FetchURL: "https://github.com/acme/widgets.git"}}
	boom := errors.New("boom")
	cases := []struct {
		name  string
		local fakeLister
		open  error
		token error
		want  ghshared.Phase
	}{
		{"no repo", fakeLister{}, fmt.Errorf("%w: /x", git.ErrNotRepo), nil, ghshared.PhaseNoRepo},
		{"open error", fakeLister{}, boom, nil, ghshared.PhaseError},
		{"remotes error", fakeLister{err: boom}, nil, nil, ghshared.PhaseError},
		{"no remote", fakeLister{}, nil, nil, ghshared.PhaseNoRemote},
		{"no gh", fakeLister{remotes: good}, nil, github.ErrNoGH, ghshared.PhaseNoGH},
		{"not logged in", fakeLister{remotes: good}, nil, github.ErrNotLoggedIn, ghshared.PhaseNotLoggedIn},
		{"token error", fakeLister{remotes: good}, nil, boom, ghshared.PhaseError},
		{"ready", fakeLister{remotes: good}, nil, nil, ghshared.PhaseReady},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			open := func(context.Context, string) (fakeLister, error) { return tc.local, tc.open }
			token := func(context.Context, string) (string, error) { return "tok", tc.token }
			res := ghshared.Resolve(context.Background(), open, token, "/work")
			if res.Phase != tc.want {
				t.Fatalf("phase %d, want %d", res.Phase, tc.want)
			}
			if tc.want == ghshared.PhaseReady && (res.Token != "tok" || res.Repo.Owner != "acme" || res.Remotes["origin"].Owner != "acme") {
				t.Errorf("ready resolution incomplete: %+v", res)
			}
			if tc.want == ghshared.PhaseNoGH && res.Repo.Owner != "acme" {
				t.Errorf("the repo must be kept to build the sign in command: %+v", res)
			}
		})
	}
}
