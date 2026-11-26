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
	issueQualifier   = "is:issue"
	searchResultCap  = 1000
	issueStateOpen   = "OPEN"
	issueStateClosed = "CLOSED"
	repoPathSegments = 2
	reposSegment     = "repos"
	milestoneNone    = 0
	apiHostPrefix    = "api."
)

type Label struct {
	Name        string
	Color       string
	Description string
}

type Milestone struct {
	Number       int
	Title        string
	Description  string
	State        string
	Due          time.Time
	OpenIssues   int
	ClosedIssues int
}

type LinkedPR struct {
	Number int
	Title  string
	URL    string
	State  string
}

type Issue struct {
	NodeID    string
	Number    int
	Title     string
	Body      string
	State     string
	Author    string
	Labels    []Label
	Assignees []string
	Milestone *Milestone
	Comments  int
	URL       string
	Created   time.Time
	Updated   time.Time
	Repo      Remote
	LinkedPRs []LinkedPR
}

type IssuePage struct {
	Issues  []Issue
	Total   int
	HasMore bool
}

type IssueInput struct {
	Title     string
	Body      string
	Labels    []string
	Assignees []string
	Milestone int
}

type IssueEdit struct {
	Title     *string
	Body      *string
	State     *string
	Labels    *[]string
	Assignees *[]string
	Milestone *int
}

func (e IssueEdit) payload() (map[string]any, error) {
	payload := map[string]any{}
	if e.Title != nil {
		if strings.TrimSpace(*e.Title) == "" {
			return nil, errors.New("github: issue title is empty")
		}
		payload["title"] = *e.Title
	}
	if e.Body != nil {
		payload["body"] = *e.Body
	}
	if e.State != nil {
		if *e.State != StateOpen && *e.State != StateClosed {
			return nil, fmt.Errorf("github: invalid issue state %q", *e.State)
		}
		payload["state"] = *e.State
	}
	if e.Labels != nil {
		payload["labels"] = nonNil(*e.Labels)
	}
	if e.Assignees != nil {
		payload["assignees"] = nonNil(*e.Assignees)
	}
	if e.Milestone != nil {
		switch {
		case *e.Milestone < milestoneNone:
			return nil, fmt.Errorf("github: invalid milestone %d", *e.Milestone)
		case *e.Milestone == milestoneNone:
			payload["milestone"] = nil
		default:
			payload["milestone"] = *e.Milestone
		}
	}
	if len(payload) == 0 {
		return nil, errors.New("github: issue edit changes nothing")
	}
	return payload, nil
}

type restLabel struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

type restMilestone struct {
	Number       int        `json:"number"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	State        string     `json:"state"`
	DueOn        *time.Time `json:"due_on"`
	OpenIssues   int        `json:"open_issues"`
	ClosedIssues int        `json:"closed_issues"`
}

type restIssue struct {
	NodeID        string         `json:"node_id"`
	Number        int            `json:"number"`
	Title         string         `json:"title"`
	Body          string         `json:"body"`
	State         string         `json:"state"`
	HTMLURL       string         `json:"html_url"`
	RepositoryURL string         `json:"repository_url"`
	User          *apiUser       `json:"user"`
	Labels        []restLabel    `json:"labels"`
	Assignees     []apiUser      `json:"assignees"`
	Milestone     *restMilestone `json:"milestone"`
	Comments      int            `json:"comments"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	PullRequest   *struct{}      `json:"pull_request"`
}

func (m restMilestone) convert() Milestone {
	out := Milestone{Number: m.Number, Title: m.Title, Description: m.Description, State: m.State, OpenIssues: m.OpenIssues, ClosedIssues: m.ClosedIssues}
	if m.DueOn != nil {
		out.Due = *m.DueOn
	}
	return out
}

func (l restLabel) convert() Label {
	return Label{Name: l.Name, Color: l.Color, Description: l.Description}
}

func (a restIssue) convert() (Issue, error) {
	if a.Number <= 0 || a.State == "" {
		return Issue{}, errors.New("github: issue response lacks number or state")
	}
	issue := Issue{
		NodeID:   a.NodeID,
		Number:   a.Number,
		Title:    a.Title,
		Body:     a.Body,
		State:    strings.ToLower(a.State),
		Author:   userLogin(a.User),
		URL:      a.HTMLURL,
		Comments: a.Comments,
		Created:  a.CreatedAt,
		Updated:  a.UpdatedAt,
		Repo:     repoFromAPIURL(a.RepositoryURL),
	}
	issue.Labels = make([]Label, 0, len(a.Labels))
	for _, l := range a.Labels {
		issue.Labels = append(issue.Labels, l.convert())
	}
	issue.Assignees = make([]string, 0, len(a.Assignees))
	for _, u := range a.Assignees {
		issue.Assignees = append(issue.Assignees, u.Login)
	}
	if a.Milestone != nil {
		m := a.Milestone.convert()
		issue.Milestone = &m
	}
	return issue, nil
}

func repoFromAPIURL(raw string) Remote {
	u, err := url.Parse(raw)
	if err != nil {
		return Remote{}
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < repoPathSegments+1 || parts[len(parts)-repoPathSegments-1] != reposSegment {
		return Remote{}
	}
	return Remote{Host: strings.TrimPrefix(strings.ToLower(u.Host), apiHostPrefix), Owner: parts[len(parts)-repoPathSegments], Name: parts[len(parts)-1]}
}

func checkIssueNumber(n int) error {
	if n <= 0 {
		return fmt.Errorf("github: invalid issue number %d", n)
	}
	return nil
}

func searchIssueQuery(query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", errors.New("github: issue search query is empty")
	}
	if !strings.Contains(query, issueQualifier) {
		query += " " + issueQualifier
	}
	return query, nil
}

func (c *Client) SearchIssues(ctx context.Context, query string, page int) (IssuePage, error) {
	q, err := searchIssueQuery(query)
	if err != nil {
		return IssuePage{}, err
	}
	if page <= 0 {
		page = 1
	}
	params := url.Values{}
	params.Set("q", q)
	params.Set("page", strconv.Itoa(page))
	params.Set("per_page", strconv.Itoa(defaultPerPage))
	var result struct {
		Total *int         `json:"total_count"`
		Items *[]restIssue `json:"items"`
	}
	if _, err := c.do(ctx, http.MethodGet, "/search/issues?"+params.Encode(), nil, &result); err != nil {
		return IssuePage{}, err
	}
	if result.Items == nil || result.Total == nil {
		return IssuePage{}, errors.New("github: issue search response lacks items")
	}
	out := IssuePage{Total: *result.Total, Issues: make([]Issue, 0, len(*result.Items))}
	for _, item := range *result.Items {
		if item.PullRequest != nil {
			continue
		}
		issue, err := item.convert()
		if err != nil {
			return IssuePage{}, err
		}
		out.Issues = append(out.Issues, issue)
	}
	fetched := min(page*defaultPerPage, searchResultCap)
	out.HasMore = len(*result.Items) == defaultPerPage && fetched < out.Total && fetched < searchResultCap
	return out, nil
}

const issueQuery = `query($owner:String!,$name:String!,$number:Int!){repository(owner:$owner,name:$name){issue(number:$number){id number title body state url createdAt updatedAt author{login} labels(first:50){nodes{name color description}} assignees(first:20){nodes{login}} milestone{number title description state dueOn} comments{totalCount} closedByPullRequestsReferences(first:20,includeClosedPrs:true){nodes{number title url state}}}}}`

type gqlIssue struct {
	ID        string    `json:"id"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Author    *apiUser  `json:"author"`
	Labels    *struct {
		Nodes []restLabel `json:"nodes"`
	} `json:"labels"`
	Assignees *struct {
		Nodes []apiUser `json:"nodes"`
	} `json:"assignees"`
	Milestone *struct {
		Number      int        `json:"number"`
		Title       string     `json:"title"`
		Description string     `json:"description"`
		State       string     `json:"state"`
		DueOn       *time.Time `json:"dueOn"`
	} `json:"milestone"`
	Comments *struct {
		TotalCount int `json:"totalCount"`
	} `json:"comments"`
	ClosedBy *struct {
		Nodes []struct {
			Number int    `json:"number"`
			Title  string `json:"title"`
			URL    string `json:"url"`
			State  string `json:"state"`
		} `json:"nodes"`
	} `json:"closedByPullRequestsReferences"`
}

func (g gqlIssue) convert(repo Remote) (Issue, error) {
	if g.Number <= 0 || g.State == "" {
		return Issue{}, errors.New("github: issue response lacks number or state")
	}
	issue := Issue{
		NodeID:  g.ID,
		Number:  g.Number,
		Title:   g.Title,
		Body:    g.Body,
		State:   strings.ToLower(g.State),
		Author:  userLogin(g.Author),
		URL:     g.URL,
		Created: g.CreatedAt,
		Updated: g.UpdatedAt,
		Repo:    repo,
	}
	if g.Labels != nil {
		for _, l := range g.Labels.Nodes {
			issue.Labels = append(issue.Labels, l.convert())
		}
	}
	if g.Assignees != nil {
		for _, u := range g.Assignees.Nodes {
			issue.Assignees = append(issue.Assignees, u.Login)
		}
	}
	if g.Milestone != nil {
		m := Milestone{Number: g.Milestone.Number, Title: g.Milestone.Title, Description: g.Milestone.Description, State: strings.ToLower(g.Milestone.State)}
		if g.Milestone.DueOn != nil {
			m.Due = *g.Milestone.DueOn
		}
		issue.Milestone = &m
	}
	if g.Comments != nil {
		issue.Comments = g.Comments.TotalCount
	}
	if g.ClosedBy != nil {
		for _, n := range g.ClosedBy.Nodes {
			issue.LinkedPRs = append(issue.LinkedPRs, LinkedPR{Number: n.Number, Title: n.Title, URL: n.URL, State: strings.ToLower(n.State)})
		}
	}
	return issue, nil
}

func (c *Client) GetIssue(ctx context.Context, repo Remote, number int) (Issue, error) {
	if _, err := repoURL(repo); err != nil {
		return Issue{}, err
	}
	if err := checkIssueNumber(number); err != nil {
		return Issue{}, err
	}
	var data struct {
		Repository *struct {
			Issue *gqlIssue `json:"issue"`
		} `json:"repository"`
	}
	vars := map[string]any{"owner": repo.Owner, "name": repo.Name, "number": number}
	if err := c.graphql(ctx, issueQuery, vars, &data); err != nil {
		return Issue{}, err
	}
	if data.Repository == nil || data.Repository.Issue == nil {
		return Issue{}, fmt.Errorf("github: issue %d not found", number)
	}
	return data.Repository.Issue.convert(repo)
}

func (c *Client) IssueComments(ctx context.Context, repo Remote, number int) ([]Comment, error) {
	if err := checkIssueNumber(number); err != nil {
		return nil, err
	}
	return c.Comments(ctx, repo, number)
}

func (c *Client) AddIssueComment(ctx context.Context, repo Remote, number int, body string) (Comment, error) {
	if err := checkIssueNumber(number); err != nil {
		return Comment{}, err
	}
	return c.AddComment(ctx, repo, number, body)
}

func (c *Client) CreateIssue(ctx context.Context, repo Remote, in IssueInput) (Issue, error) {
	base, err := repoURL(repo)
	if err != nil {
		return Issue{}, err
	}
	if strings.TrimSpace(in.Title) == "" {
		return Issue{}, errors.New("github: issue title is empty")
	}
	if in.Milestone < milestoneNone {
		return Issue{}, fmt.Errorf("github: invalid milestone %d", in.Milestone)
	}
	payload := map[string]any{"title": in.Title}
	if in.Body != "" {
		payload["body"] = in.Body
	}
	if len(in.Labels) > 0 {
		payload["labels"] = in.Labels
	}
	if len(in.Assignees) > 0 {
		payload["assignees"] = in.Assignees
	}
	if in.Milestone > milestoneNone {
		payload["milestone"] = in.Milestone
	}
	var raw restIssue
	if _, err := c.do(ctx, http.MethodPost, base+"/issues", payload, &raw); err != nil {
		return Issue{}, err
	}
	return raw.convert()
}

func (c *Client) EditIssue(ctx context.Context, repo Remote, number int, edit IssueEdit) (Issue, error) {
	base, err := repoURL(repo)
	if err != nil {
		return Issue{}, err
	}
	if err := checkIssueNumber(number); err != nil {
		return Issue{}, err
	}
	payload, err := edit.payload()
	if err != nil {
		return Issue{}, err
	}
	var raw restIssue
	if _, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("%s/issues/%d", base, number), payload, &raw); err != nil {
		return Issue{}, err
	}
	return raw.convert()
}

func (c *Client) Milestones(ctx context.Context, repo Remote) ([]Milestone, error) {
	base, err := repoURL(repo)
	if err != nil {
		return nil, err
	}
	raw, err := listAll[restMilestone](ctx, c, base+"/milestones?state=open&per_page="+maxPerPageText)
	if err != nil {
		return nil, err
	}
	milestones := make([]Milestone, 0, len(raw))
	for _, m := range raw {
		if m.Number <= 0 || m.Title == "" {
			return nil, errors.New("github: milestone lacks number or title")
		}
		milestones = append(milestones, m.convert())
	}
	return milestones, nil
}

func (c *Client) Labels(ctx context.Context, repo Remote) ([]Label, error) {
	base, err := repoURL(repo)
	if err != nil {
		return nil, err
	}
	raw, err := listAll[restLabel](ctx, c, base+"/labels?per_page="+maxPerPageText)
	if err != nil {
		return nil, err
	}
	labels := make([]Label, 0, len(raw))
	for _, l := range raw {
		if l.Name == "" {
			return nil, errors.New("github: label lacks name")
		}
		labels = append(labels, l.convert())
	}
	return labels, nil
}
