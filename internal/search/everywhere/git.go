package everywhere

import (
	"cmp"
	"context"
	"regexp"
	"slices"
	"strings"

	"github.com/ZerGo0/editor/internal/fuzzy"
	"github.com/ZerGo0/editor/internal/git"
)

const (
	gitCommitLimit = 50
	shortHashLen   = 8
	minHashQuery   = 4
	dateLayout     = "2006-01-02"
)

var hashQuery = regexp.MustCompile("^[0-9a-fA-F]+$")

type GitRef struct {
	Name   string
	Remote bool
	Tag    bool
}

func (s *Session) Git(ctx context.Context, q string, limit int, commits bool) ([]Result, int) {
	if s.env.Repo == nil || q == "" && !commits {
		return nil, 0
	}
	repo := s.env.Repo()
	if repo == nil {
		return nil, 0
	}
	out := filterRefs(q, s.loadRefs(ctx, repo))
	if q == "" {
		return out[:min(len(out), limit)], len(out)
	}
	if !commits {
		return out[:min(len(out), limit)], len(out)
	}
	opts := git.LogOptions{Max: gitCommitLimit, All: true, Grep: q}
	if hashQuery.MatchString(q) && len(q) >= minHashQuery {
		opts = git.LogOptions{Max: 1, Revs: []string{q}}
	}
	if log, err := repo.Log(ctx, opts); err == nil {
		for _, c := range log {
			out = append(out, Result{
				Kind:   KindGitCommit,
				Label:  c.Subject,
				Detail: c.OID[:min(len(c.OID), shortHashLen)] + "  " + c.Author + "  " + c.AuthorTime.Format(dateLayout),
				OID:    c.OID,
			})
		}
	}
	return out[:min(len(out), limit)], len(out)
}

func (s *Session) loadRefs(ctx context.Context, repo *git.Repo) []Result {
	s.refsOnce.Do(func() {
		if branches, err := repo.Branches(ctx); err == nil {
			for _, b := range branches {
				detail := "local branch"
				switch {
				case b.Current:
					detail = "current branch"
				case b.Remote:
					detail = "remote branch"
				}
				s.refs = append(s.refs, Result{Kind: KindGitRef, Label: b.Name, Detail: detail, Ref: GitRef{Name: b.Name, Remote: b.Remote}})
			}
		}
		if tags, err := repo.Tags(ctx); err == nil {
			for _, t := range tags {
				s.refs = append(s.refs, Result{Kind: KindGitRef, Label: t.Name, Detail: "tag", Ref: GitRef{Name: t.Name, Tag: true}})
			}
		}
	})
	return s.refs
}

func filterRefs(q string, refs []Result) []Result {
	if q == "" {
		return slices.Clone(refs)
	}
	type scored struct {
		result Result
		score  int
	}
	var hits []scored
	for _, r := range refs {
		if m, ok := fuzzy.Find(q, r.Label); ok {
			r.Positions = m.Positions
			hits = append(hits, scored{r, m.Score})
		}
	}
	slices.SortStableFunc(hits, func(a, b scored) int {
		return cmp.Or(cmp.Compare(b.score, a.score), strings.Compare(a.result.Label, b.result.Label))
	})
	out := make([]Result, len(hits))
	for i, h := range hits {
		out[i] = h.result
	}
	return out
}
