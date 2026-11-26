package issueui

import (
	"context"
	"slices"
	"sync"

	"github.com/ZerGo0/editor/internal/github"
)

const maxUserPages = 5

type metaState struct {
	labels     []github.Label
	milestones []github.Milestone
	users      []github.User
	loading    bool
	loaded     bool
	err        error
	gen        int
}

type metaResult struct {
	labels     []github.Label
	milestones []github.Milestone
	users      []github.User
	err        error
}

func collectUsers(ctx context.Context, fetch func(ctx context.Context, after string) (github.UserPage, error)) ([]github.User, error) {
	var users []github.User
	after := ""
	for range maxUserPages {
		page, err := fetch(ctx, after)
		if err != nil {
			return users, err
		}
		users = append(users, page.Users...)
		if page.Next == "" {
			break
		}
		after = page.Next
	}
	return users, nil
}

func fetchMeta(ctx context.Context, be Backend, repo github.Remote) metaResult {
	var res metaResult
	var mu sync.Mutex
	var wg sync.WaitGroup
	fail := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if res.err == nil {
			res.err = err
		}
	}
	run := func(fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fail(fn())
		}()
	}
	run(func() (err error) { res.labels, err = be.Labels(ctx, repo); return })
	run(func() (err error) { res.milestones, err = be.Milestones(ctx, repo); return })
	run(func() (err error) {
		res.users, err = collectUsers(ctx, func(ctx context.Context, after string) (github.UserPage, error) {
			return be.AssignableUsers(ctx, repo, after)
		})
		return
	})
	wg.Wait()
	return res
}

func (i *Issues) ensureMeta() {
	if !i.meta.loaded && !i.meta.loading {
		i.loadMeta()
	}
}

func (i *Issues) loadMeta() {
	i.meta.gen++
	gen := i.meta.gen
	i.meta.loading, i.meta.err = true, nil
	be, repo := i.be, i.repo
	go func() {
		res := fetchMeta(i.ctx, be, repo)
		i.post(func() {
			if gen != i.meta.gen {
				return
			}
			i.meta.loading, i.meta.loaded, i.meta.err = false, true, res.err
			i.meta.labels, i.meta.milestones, i.meta.users = res.labels, res.milestones, res.users
			i.authFailure(res.err)
		})
	}()
}

func (m metaState) labelNames() []string {
	names := make([]string, 0, len(m.labels))
	for _, l := range m.labels {
		names = append(names, l.Name)
	}
	return names
}

func (m metaState) userLogins() []string {
	logins := make([]string, 0, len(m.users))
	for _, u := range m.users {
		logins = append(logins, u.Login)
	}
	return logins
}

func (m metaState) milestoneOptions() []string {
	options := []string{noMilestoneOption}
	for _, ms := range m.milestones {
		options = append(options, ms.Title)
	}
	return options
}

func (m metaState) milestoneNumber(title string) int {
	if at := slices.IndexFunc(m.milestones, func(ms github.Milestone) bool { return ms.Title == title }); at >= 0 {
		return m.milestones[at].Number
	}
	return 0
}
