package ghshared_test

import (
	"testing"

	"github.com/ZerGo0/editor/internal/ghshared"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
)

func TestPickRemote(t *testing.T) {
	remotes := []git.Remote{
		{Name: "zed", FetchURL: "git@github.com:zed/widgets.git"},
		{Name: "origin", FetchURL: "https://github.com/acme/widgets.git"},
		{Name: "gitlab", FetchURL: "git@gitlab.com:acme/widgets.git"},
		{Name: "local", FetchURL: "/srv/repo.git"},
	}
	repo, byName, ok := ghshared.PickRemote(remotes)
	if !ok || repo.Owner != "acme" || repo.Host != "github.com" || len(byName) != 3 {
		t.Fatalf("%+v %v %v", repo, byName, ok)
	}
	if got := ghshared.RemoteNameFor(byName, repo); got != "origin" {
		t.Errorf("remote name %q", got)
	}
	upstream := []git.Remote{
		{Name: "local", FetchURL: "/srv/repo.git"},
		{Name: "fork", FetchURL: "git@github.com:me/widgets.git"},
		{Name: "upstream", FetchURL: "git@github.com:acme/widgets.git"},
	}
	if repo, _, ok := ghshared.PickRemote(upstream); !ok || repo.Owner != "acme" {
		t.Errorf("upstream must beat a fork: %+v %v", repo, ok)
	}
	if repo, _, ok := ghshared.PickRemote(upstream[1:2]); !ok || repo.Owner != "me" {
		t.Errorf("fallback remote: %+v %v", repo, ok)
	}
	pushOnly := []git.Remote{{Name: "origin", FetchURL: "not a url", PushURL: "git@github.com:me/widgets.git"}}
	if repo, _, ok := ghshared.PickRemote(pushOnly); !ok || repo.Owner != "me" {
		t.Errorf("push URL fallback: %+v %v", repo, ok)
	}
	for _, none := range [][]git.Remote{nil, {{Name: "x", FetchURL: "not a url"}}, upstream[:1]} {
		if _, _, ok := ghshared.PickRemote(none); ok {
			t.Errorf("%+v accepted", none)
		}
	}
}

func TestRemoteNameForIsDeterministic(t *testing.T) {
	remotes := map[string]github.Remote{"b": {Owner: "x"}, "a": {Owner: "x"}}
	if got := ghshared.RemoteNameFor(remotes, github.Remote{Owner: "x"}); got != "a" {
		t.Errorf("remote name %q, want a", got)
	}
	if got := ghshared.RemoteNameFor(remotes, github.Remote{Owner: "y"}); got != ghshared.OriginRemote {
		t.Errorf("unknown repo: remote name %q, want %s", got, ghshared.OriginRemote)
	}
}
