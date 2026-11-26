package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	StateOpen   = "open"
	StateClosed = "closed"
	StateAll    = "all"

	defaultPerPage = 30
	maxPerPage     = 100
	maxPerPageText = "100"

	checkStatusCompleted  = "completed"
	checkStatusInProgress = "in_progress"
	commitStatusPending   = "pending"
)

type PullRequest struct {
	NodeID       string
	Number       int
	Title        string
	Body         string
	State        string
	URL          string
	Author       string
	HeadRef      string
	HeadOwner    string
	HeadRepo     string
	HeadSHA      string
	BaseRef      string
	Draft        bool
	Merged       bool
	Mergeable    *bool
	Labels       []string
	Reviewers    []string
	Created      time.Time
	Updated      time.Time
	Additions    int
	Deletions    int
	ChangedFiles int
	Comments     int
}

type ListOptions struct {
	State           string
	Author          string
	Assignee        string
	ReviewRequested string
	Query           string
	Page            int
	PerPage         int
}

func (o ListOptions) needsSearch() bool {
	return o.Query != "" || o.Author != "" || o.Assignee != "" || o.ReviewRequested != ""
}

type PRFile struct {
	Path      string
	PrevPath  string
	Status    string
	Additions int
	Deletions int
	Patch     string
}

type Commit struct {
	SHA     string
	Message string
	Author  string
	Date    time.Time
	URL     string
}

type Check struct {
	Name       string
	Status     string
	Conclusion string
	URL        string
}

type apiUser struct {
	Login string `json:"login"`
}

type apiLabel struct {
	Name string `json:"name"`
}

type apiHead struct {
	Ref  string   `json:"ref"`
	SHA  string   `json:"sha"`
	User *apiUser `json:"user"`
	Repo *struct {
		Name  string   `json:"name"`
		Owner *apiUser `json:"owner"`
	} `json:"repo"`
}

type apiPull struct {
	NodeID             string     `json:"node_id"`
	Number             int        `json:"number"`
	Title              string     `json:"title"`
	Body               string     `json:"body"`
	State              string     `json:"state"`
	HTMLURL            string     `json:"html_url"`
	Draft              bool       `json:"draft"`
	Merged             bool       `json:"merged"`
	MergedAt           *time.Time `json:"merged_at"`
	Mergeable          *bool      `json:"mergeable"`
	User               *apiUser   `json:"user"`
	Head               *apiHead   `json:"head"`
	Base               *apiHead   `json:"base"`
	Labels             []apiLabel `json:"labels"`
	RequestedReviewers []apiUser  `json:"requested_reviewers"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	Additions          int        `json:"additions"`
	Deletions          int        `json:"deletions"`
	ChangedFiles       int        `json:"changed_files"`
	Comments           int        `json:"comments"`
}

type apiIssue struct {
	NodeID      string     `json:"node_id"`
	Number      int        `json:"number"`
	Title       string     `json:"title"`
	Body        string     `json:"body"`
	State       string     `json:"state"`
	HTMLURL     string     `json:"html_url"`
	Draft       bool       `json:"draft"`
	User        *apiUser   `json:"user"`
	Labels      []apiLabel `json:"labels"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Comments    int        `json:"comments"`
	PullRequest *struct {
		MergedAt *time.Time `json:"merged_at"`
	} `json:"pull_request"`
}

type apiFile struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
	Status           string `json:"status"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	Patch            string `json:"patch"`
}

type apiCommit struct {
	SHA     string `json:"sha"`
	HTMLURL string `json:"html_url"`
	Commit  *struct {
		Message string `json:"message"`
		Author  *struct {
			Name string    `json:"name"`
			Date time.Time `json:"date"`
		} `json:"author"`
	} `json:"commit"`
	Author *apiUser `json:"author"`
}

func userLogin(u *apiUser) string {
	if u == nil {
		return ""
	}
	return u.Login
}

func labelNames(labels []apiLabel) []string {
	names := make([]string, 0, len(labels))
	for _, l := range labels {
		names = append(names, l.Name)
	}
	return names
}

func (a apiPull) convert() (PullRequest, error) {
	if a.Number <= 0 || a.State == "" {
		return PullRequest{}, errors.New("github: pull request response lacks number or state")
	}
	if a.Head == nil || a.Base == nil {
		return PullRequest{}, fmt.Errorf("github: pull request %d response lacks head or base", a.Number)
	}
	pr := PullRequest{
		NodeID:       a.NodeID,
		Number:       a.Number,
		Title:        a.Title,
		Body:         a.Body,
		State:        a.State,
		URL:          a.HTMLURL,
		Author:       userLogin(a.User),
		HeadRef:      a.Head.Ref,
		HeadSHA:      a.Head.SHA,
		BaseRef:      a.Base.Ref,
		Draft:        a.Draft,
		Merged:       a.Merged || a.MergedAt != nil,
		Mergeable:    a.Mergeable,
		Labels:       labelNames(a.Labels),
		Created:      a.CreatedAt,
		Updated:      a.UpdatedAt,
		Additions:    a.Additions,
		Deletions:    a.Deletions,
		ChangedFiles: a.ChangedFiles,
		Comments:     a.Comments,
	}
	if a.Head.Repo != nil {
		pr.HeadRepo = a.Head.Repo.Name
		pr.HeadOwner = userLogin(a.Head.Repo.Owner)
	}
	if pr.HeadOwner == "" {
		pr.HeadOwner = userLogin(a.Head.User)
	}
	pr.Reviewers = make([]string, 0, len(a.RequestedReviewers))
	for _, r := range a.RequestedReviewers {
		pr.Reviewers = append(pr.Reviewers, r.Login)
	}
	return pr, nil
}

func (a apiIssue) convert() (PullRequest, bool, error) {
	if a.PullRequest == nil {
		return PullRequest{}, false, nil
	}
	if a.Number <= 0 || a.State == "" {
		return PullRequest{}, false, errors.New("github: search result lacks number or state")
	}
	return PullRequest{
		NodeID:   a.NodeID,
		Number:   a.Number,
		Title:    a.Title,
		Body:     a.Body,
		State:    a.State,
		URL:      a.HTMLURL,
		Author:   userLogin(a.User),
		Draft:    a.Draft,
		Merged:   a.PullRequest.MergedAt != nil,
		Labels:   labelNames(a.Labels),
		Created:  a.CreatedAt,
		Updated:  a.UpdatedAt,
		Comments: a.Comments,
	}, true, nil
}

func normalizeState(state string) (string, error) {
	switch state {
	case "":
		return StateOpen, nil
	case StateOpen, StateClosed, StateAll:
		return state, nil
	}
	return "", fmt.Errorf("github: invalid state %q", state)
}

func paging(o ListOptions) (page, perPage int) {
	page, perPage = o.Page, o.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = defaultPerPage
	}
	return page, min(perPage, maxPerPage)
}

func (c *Client) List(ctx context.Context, repo Remote, opts ListOptions) ([]PullRequest, error) {
	base, err := repoURL(repo)
	if err != nil {
		return nil, err
	}
	state, err := normalizeState(opts.State)
	if err != nil {
		return nil, err
	}
	page, perPage := paging(opts)
	params := url.Values{}
	params.Set("page", strconv.Itoa(page))
	params.Set("per_page", strconv.Itoa(perPage))

	if opts.needsSearch() {
		params.Set("q", searchQuery(repo, state, opts))
		return c.search(ctx, "/search/issues?"+params.Encode())
	}
	params.Set("state", state)
	var raw []apiPull
	if _, err := c.do(ctx, http.MethodGet, base+"/pulls?"+params.Encode(), nil, &raw); err != nil {
		return nil, err
	}
	prs := make([]PullRequest, 0, len(raw))
	for _, item := range raw {
		pr, err := item.convert()
		if err != nil {
			return nil, err
		}
		prs = append(prs, pr)
	}
	return prs, nil
}

func searchQuery(repo Remote, state string, opts ListOptions) string {
	terms := []string{"repo:" + repo.Owner + "/" + repo.Name, "is:pr"}
	if state != StateAll {
		terms = append(terms, "is:"+state)
	}
	if opts.Author != "" {
		terms = append(terms, "author:"+opts.Author)
	}
	if opts.Assignee != "" {
		terms = append(terms, "assignee:"+opts.Assignee)
	}
	if opts.ReviewRequested != "" {
		terms = append(terms, "review-requested:"+opts.ReviewRequested)
	}
	if opts.Query != "" {
		terms = append(terms, opts.Query)
	}
	return strings.Join(terms, " ")
}

func (c *Client) search(ctx context.Context, path string) ([]PullRequest, error) {
	var result struct {
		Items *[]apiIssue `json:"items"`
	}
	if _, err := c.do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	if result.Items == nil {
		return nil, errors.New("github: search response lacks items")
	}
	prs := make([]PullRequest, 0, len(*result.Items))
	for _, item := range *result.Items {
		pr, ok, err := item.convert()
		if err != nil {
			return nil, err
		}
		if ok {
			prs = append(prs, pr)
		}
	}
	return prs, nil
}

func (c *Client) Get(ctx context.Context, repo Remote, number int) (PullRequest, error) {
	base, err := repoURL(repo)
	if err != nil {
		return PullRequest{}, err
	}
	if err := checkNumber(number); err != nil {
		return PullRequest{}, err
	}
	var raw apiPull
	if _, err := c.do(ctx, http.MethodGet, fmt.Sprintf("%s/pulls/%d", base, number), nil, &raw); err != nil {
		return PullRequest{}, err
	}
	return raw.convert()
}

func (c *Client) CurrentBranchPR(ctx context.Context, repo Remote, branch, headOwner string) (*PullRequest, error) {
	base, err := repoURL(repo)
	if err != nil {
		return nil, err
	}
	if branch == "" || headOwner == "" {
		return nil, errors.New("github: branch and head owner are required")
	}
	params := url.Values{}
	params.Set("head", headOwner+":"+branch)
	params.Set("state", StateOpen)
	var raw []apiPull
	if _, err := c.do(ctx, http.MethodGet, base+"/pulls?"+params.Encode(), nil, &raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	pr, err := raw[0].convert()
	if err != nil {
		return nil, err
	}
	return &pr, nil
}

func (c *Client) Files(ctx context.Context, repo Remote, number int) ([]PRFile, error) {
	path, err := pullSubpath(repo, number, "files")
	if err != nil {
		return nil, err
	}
	raw, err := listAll[apiFile](ctx, c, path)
	if err != nil {
		return nil, err
	}
	files := make([]PRFile, 0, len(raw))
	for _, f := range raw {
		if f.Filename == "" {
			return nil, errors.New("github: file entry lacks filename")
		}
		files = append(files, PRFile{Path: f.Filename, PrevPath: f.PreviousFilename, Status: f.Status, Additions: f.Additions, Deletions: f.Deletions, Patch: f.Patch})
	}
	return files, nil
}

func (c *Client) Commits(ctx context.Context, repo Remote, number int) ([]Commit, error) {
	path, err := pullSubpath(repo, number, "commits")
	if err != nil {
		return nil, err
	}
	raw, err := listAll[apiCommit](ctx, c, path)
	if err != nil {
		return nil, err
	}
	commits := make([]Commit, 0, len(raw))
	for _, item := range raw {
		if item.SHA == "" || item.Commit == nil {
			return nil, errors.New("github: commit entry lacks sha or commit")
		}
		commit := Commit{SHA: item.SHA, Message: item.Commit.Message, URL: item.HTMLURL, Author: userLogin(item.Author)}
		if item.Commit.Author != nil {
			commit.Date = item.Commit.Author.Date
			if commit.Author == "" {
				commit.Author = item.Commit.Author.Name
			}
		}
		commits = append(commits, commit)
	}
	return commits, nil
}

func (c *Client) Checks(ctx context.Context, repo Remote, sha string) ([]Check, error) {
	base, err := repoURL(repo)
	if err != nil {
		return nil, err
	}
	if sha == "" {
		return nil, errors.New("github: commit sha is required")
	}
	ref := base + "/commits/" + url.PathEscape(sha)
	var checks []Check
	type runPage struct {
		CheckRuns []struct {
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			HTMLURL    string `json:"html_url"`
		} `json:"check_runs"`
	}
	var runErr error
	err = getPages(ctx, c, ref+"/check-runs?per_page="+maxPerPageText, func(p runPage) {
		for _, r := range p.CheckRuns {
			if r.Name == "" {
				runErr = errors.New("github: check run lacks name")
			}
			checks = append(checks, Check{Name: r.Name, Status: r.Status, Conclusion: r.Conclusion, URL: r.HTMLURL})
		}
	})
	if err == nil {
		err = runErr
	}
	if err != nil {
		return nil, err
	}
	type statusPage struct {
		Statuses []struct {
			Context   string `json:"context"`
			State     string `json:"state"`
			TargetURL string `json:"target_url"`
		} `json:"statuses"`
	}
	err = getPages(ctx, c, ref+"/status?per_page="+maxPerPageText, func(p statusPage) {
		for _, s := range p.Statuses {
			if s.Context == "" {
				runErr = errors.New("github: commit status lacks context")
			}
			check := Check{Name: s.Context, Status: checkStatusCompleted, Conclusion: s.State, URL: s.TargetURL}
			if s.State == commitStatusPending {
				check.Status, check.Conclusion = checkStatusInProgress, ""
			}
			checks = append(checks, check)
		}
	})
	if err == nil {
		err = runErr
	}
	if err != nil {
		return nil, err
	}
	return checks, nil
}

func pullSubpath(repo Remote, number int, suffix string) (string, error) {
	base, err := repoURL(repo)
	if err != nil {
		return "", err
	}
	if err := checkNumber(number); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/pulls/%d/%s?per_page=%s", base, number, suffix, maxPerPageText), nil
}
