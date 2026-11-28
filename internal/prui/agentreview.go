package prui

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/multidiff"
	"github.com/ZerGo0/editor/internal/review"
)

type AgentReview struct {
	p        *PRs
	repo     github.Remote
	number   int
	decorate func([]multidiff.Entry) []multidiff.Entry
	view     *multidiff.View
	viewed   *multidiff.Memory
	pr       github.PullRequest
	loaded   bool
	loading  bool
	err      error
	gen      int
	mu       sync.Mutex
	base     string
}

func (p *PRs) AgentReview(ref review.PRRef, decorate func([]multidiff.Entry) []multidiff.Entry) *AgentReview {
	ar := &AgentReview{p: p, repo: github.Remote{Host: ref.Host, Owner: ref.Owner, Name: ref.Repo}, number: ref.Number,
		decorate: decorate, view: multidiff.New(), viewed: multidiff.NewMemory()}
	ar.Reload()
	return ar
}

func (ar *AgentReview) Reload() {
	p := ar.p
	ar.gen++
	gen, repo, number := ar.gen, ar.repo, ar.number
	ar.loading, ar.err = true, nil
	var local Local
	if p.local != nil && p.repo == repo {
		local = p.local
	}
	applog.Go(func() {
		pr, files, be, err := ar.fetch(p.Ctx, repo, number)
		p.Post(func() {
			if gen != ar.gen {
				return
			}
			ar.loading, ar.loaded, ar.err = false, true, err
			if err != nil {
				return
			}
			ar.mu.Lock()
			ar.pr, ar.base = pr, ""
			ar.mu.Unlock()
			src := fileSource{local: local, be: be, repo: repo, root: p.Host.RootDir()}
			entries := make([]multidiff.Entry, 0, len(files))
			for _, f := range files {
				entries = append(entries, ar.entry(src, pr, f))
			}
			if ar.decorate != nil {
				entries = ar.decorate(entries)
			}
			ar.view.SetEntries(entries)
		})
	})
}

func (ar *AgentReview) fetch(ctx context.Context, repo github.Remote, number int) (github.PullRequest, []github.PRFile, Backend, error) {
	token, err := ar.p.env.Token(ctx, repo.Host)
	if err != nil {
		return github.PullRequest{}, nil, nil, fmt.Errorf("sign in to %s: %w", repo.Host, err)
	}
	be := ar.p.env.NewBackend(repo.Host, token)
	pr, err := be.Get(ctx, repo, number)
	if err != nil {
		return pr, nil, nil, err
	}
	files, err := be.Files(ctx, repo, number)
	return pr, files, be, err
}

func (ar *AgentReview) entry(src fileSource, pr github.PullRequest, f github.PRFile) multidiff.Entry {
	return multidiff.Entry{
		Key: f.Path, Path: f.Path, OldPath: f.PrevPath, Status: fileGitStatus(f.Status), Adds: f.Additions, Dels: f.Deletions,
		Fingerprint: pr.HeadSHA,
		Load: func(ctx context.Context, _ bool) (string, string, error) {
			ar.mu.Lock()
			base := ar.base
			if base != "" {
				ar.mu.Unlock()
			}
			v, err := loadVersions(ctx, src, pr, f, diffSpec{mode: modePR}, base)
			if base == "" {
				if err == nil {
					ar.base = v.baseSHA
				}
				ar.mu.Unlock()
			}
			if errors.Is(err, ErrBinaryFile) || errors.Is(err, ErrFileTooLarge) {
				return "", "", multidiff.Note(err.Error())
			}
			return v.old, v.new, err
		},
	}
}

func (ar *AgentReview) View() *multidiff.View { return ar.view }

func (ar *AgentReview) Options() multidiff.Options {
	o := ar.p.diffOptions(ar.Title(), "agent-review:"+ar.Title())
	o.Viewed, o.Busy = ar.viewed, ar.loading
	return o
}

func (ar *AgentReview) Title() string {
	return fmt.Sprintf("%s/%s#%d", ar.repo.Owner, ar.repo.Name, ar.number)
}

func (ar *AgentReview) PR() review.PR {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	return review.PR{Repo: ar.repo.Owner + "/" + ar.repo.Name, Number: ar.number, Title: ar.pr.Title, Head: ar.pr.HeadRef, Base: ar.pr.BaseRef, URL: ar.pr.URL}
}

func (ar *AgentReview) Status() (loaded, loading bool, err error) {
	return ar.loaded, ar.loading, ar.err
}

func (ar *AgentReview) Close() {
	ar.gen++
	ar.view.Close()
}
