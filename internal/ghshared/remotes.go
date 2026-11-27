package ghshared

import (
	"slices"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
)

const (
	OriginRemote   = "origin"
	UpstreamRemote = "upstream"
)

var preferredRemotes = []string{OriginRemote, UpstreamRemote}

func PickRemote(remotes []git.Remote) (github.Remote, map[string]github.Remote, bool) {
	byName := map[string]github.Remote{}
	for _, r := range remotes {
		for _, raw := range []string{r.FetchURL, r.PushURL} {
			if gr, ok := github.ParseRemote(raw); ok {
				byName[r.Name] = gr
				break
			}
		}
	}
	for _, name := range preferredRemotes {
		if gr, ok := byName[name]; ok {
			return gr, byName, true
		}
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	slices.Sort(names)
	if len(names) == 0 {
		return github.Remote{}, byName, false
	}
	return byName[names[0]], byName, true
}

func RemoteNameFor(remotes map[string]github.Remote, repo github.Remote) string {
	for _, name := range preferredRemotes {
		if r, ok := remotes[name]; ok && r == repo {
			return name
		}
	}
	names := make([]string, 0, len(remotes))
	for name, r := range remotes {
		if r == repo {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	if len(names) > 0 {
		return names[0]
	}
	return OriginRemote
}
