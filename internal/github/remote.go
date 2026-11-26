package github

import (
	"net/url"
	"strings"
)

const (
	gitSuffix    = ".git"
	scpSeparator = ":"
	repoSegments = 2
)

type Remote struct {
	Host  string
	Owner string
	Name  string
}

func ParseRemote(raw string) (Remote, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Remote{}, false
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return Remote{}, false
		}
		host := u.Hostname()
		if u.Scheme == "http" || u.Scheme == "https" {
			host = u.Host
		}
		return newRemote(host, u.Path)
	}
	hostPart, path, found := strings.Cut(raw, scpSeparator)
	if !found {
		return Remote{}, false
	}
	if at := strings.LastIndex(hostPart, "@"); at >= 0 {
		hostPart = hostPart[at+1:]
	}
	return newRemote(hostPart, path)
}

func newRemote(host, path string) (Remote, bool) {
	path = strings.TrimSuffix(strings.Trim(path, "/"), gitSuffix)
	parts := strings.Split(path, "/")
	if host == "" || len(parts) != repoSegments || parts[0] == "" || parts[1] == "" {
		return Remote{}, false
	}
	return Remote{Host: strings.ToLower(host), Owner: parts[0], Name: parts[1]}, true
}
