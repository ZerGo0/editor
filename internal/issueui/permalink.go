package issueui

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/ghshared"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
)

const (
	permalinkBlob       = "blob"
	permalinkLinePrefix = "#L"
	permalinkRangeSep   = "-L"
	untrackedMark       = '?'
	ignoredMark         = '!'
)

var (
	errPermalinkOutside   = errors.New("The file is outside the Git repository.")
	errPermalinkUntracked = errors.New("The file is not tracked by Git.")
	errPermalinkModified  = errors.New("The file has local changes. Commit or discard them first.")
	errPermalinkUnpushed  = errors.New("The commit is not on a remote branch. Push it first.")
	errPermalinkNoRemote  = errors.New("No GitHub remote found for this repository.")
	errPermalinkNoCommit  = errors.New("The repository has no commits yet.")
)

func repoRelative(root, path string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || !filepath.IsLocal(rel) {
		return "", errPermalinkOutside
	}
	return filepath.ToSlash(rel), nil
}

func fileStatusError(files []git.FileStatus, rel string) error {
	for _, f := range files {
		if f.Path != rel && f.OrigPath != rel {
			continue
		}
		if f.Index == untrackedMark || f.Index == ignoredMark {
			return errPermalinkUntracked
		}
		return errPermalinkModified
	}
	return nil
}

func permalinkRemote(remotes []git.Remote, upstream string) (github.Remote, bool) {
	names := []string{}
	if name, _, ok := strings.Cut(upstream, "/"); ok && name != "" {
		names = append(names, name)
	}
	names = append(names, ghshared.OriginRemote, ghshared.UpstreamRemote)
	for _, name := range names {
		if i := slices.IndexFunc(remotes, func(r git.Remote) bool { return r.Name == name }); i >= 0 {
			if remote, ok := github.ParseRemote(remotes[i].FetchURL); ok {
				return remote, true
			}
		}
	}
	for _, r := range remotes {
		if remote, ok := github.ParseRemote(r.FetchURL); ok {
			return remote, true
		}
	}
	return github.Remote{}, false
}

func permalinkURL(repo github.Remote, sha, rel string, start, end int) string {
	segments := strings.Split(rel, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	link := fmt.Sprintf("https://%s/%s/%s/%s/%s/%s", repo.Host, url.PathEscape(repo.Owner), url.PathEscape(repo.Name), permalinkBlob, sha, strings.Join(segments, "/"))
	if start < 0 {
		return link
	}
	link += permalinkLinePrefix + fmt.Sprint(start+1)
	if end > start {
		link += permalinkRangeSep + fmt.Sprint(end+1)
	}
	return link
}

func buildPermalink(ctx context.Context, local Local, path string, startLine, endLine int) (string, error) {
	if startLine < 0 {
		startLine, endLine = -1, -1
	}
	if endLine < startLine {
		startLine, endLine = endLine, startLine
	}
	rel, err := repoRelative(local.WorkDir(), path)
	if err != nil {
		return "", err
	}
	status, err := local.Status(ctx)
	if err != nil {
		return "", fmt.Errorf("read Git status: %w", err)
	}
	if status.OID == "" {
		return "", errPermalinkNoCommit
	}
	if err := fileStatusError(status.Files, rel); err != nil {
		return "", err
	}
	outgoing, err := local.OutgoingCommits(ctx)
	if err != nil {
		return "", fmt.Errorf("check remote branches: %w", err)
	}
	if len(outgoing) > 0 {
		return "", errPermalinkUnpushed
	}
	remotes, err := local.Remotes(ctx)
	if err != nil {
		return "", fmt.Errorf("read Git remotes: %w", err)
	}
	repo, ok := permalinkRemote(remotes, status.Upstream)
	if !ok {
		return "", errPermalinkNoRemote
	}
	return permalinkURL(repo, status.OID, rel, startLine, endLine), nil
}

func (i *Issues) Permalink(path string, startLine, endLine int) (string, error) {
	local, err := i.env.OpenLocal(i.Ctx, i.Host.RootDir())
	if err != nil {
		return "", fmt.Errorf("open repository: %w", err)
	}
	return buildPermalink(i.Ctx, local, path, startLine, endLine)
}

func (i *Issues) permalinkAsync(path string, startLine, endLine int, done func(link string, err error)) {
	applog.Go(func() {
		link, err := i.Permalink(path, startLine, endLine)
		i.Post(func() { done(link, err) })
	})
}

func (i *Issues) CopyPermalink(path string, startLine, endLine int) {
	i.permalinkAsync(path, startLine, endLine, func(link string, err error) {
		if err != nil {
			i.Host.Notify(err.Error())
			return
		}
		i.env.Copy(link)
		i.Host.Notify("Copied GitHub permalink")
	})
}

func (i *Issues) OpenPermalink(path string, startLine, endLine int) {
	i.permalinkAsync(path, startLine, endLine, func(link string, err error) {
		if err != nil {
			i.Host.Notify(err.Error())
			return
		}
		if err := i.env.OpenURL(link); err != nil {
			i.Host.ShowError("Cannot open the permalink", err)
		}
	})
}
