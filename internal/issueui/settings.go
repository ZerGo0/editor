package issueui

import "slices"

const (
	defaultBranchTitle    = "${user}/issue${issueNumber}"
	defaultCommitTemplate = "${issueTitle}\nFixes #${issueNumber}"
	groupByMilestone      = "milestone"

	queryMyIssuesLabel = "My Issues"
	queryCreatedLabel  = "Created Issues"
	queryRecentLabel   = "Recent Issues"
	queryMyIssues      = "is:open assignee:${user} repo:${owner}/${repository}"
	queryCreated       = "author:${user} state:open repo:${owner}/${repository} sort:created-desc"
	queryRecent        = "state:open repo:${owner}/${repository} sort:updated-desc"
)

type Query struct {
	Label   string   `json:"label"`
	Query   string   `json:"query"`
	GroupBy []string `json:"groupBy,omitempty"`
}

type Settings struct {
	IssueBranchTitle      string
	AssignWhenWorking     bool
	WorkingIssueFormatSCM string
	Queries               []Query
	LinkBranchOnGitHub    bool
}

func DefaultQueries() []Query {
	return []Query{
		{Label: queryMyIssuesLabel, Query: queryMyIssues, GroupBy: []string{groupByMilestone}},
		{Label: queryCreatedLabel, Query: queryCreated},
		{Label: queryRecentLabel, Query: queryRecent},
	}
}

func DefaultSettings() Settings {
	return Settings{
		IssueBranchTitle:      defaultBranchTitle,
		AssignWhenWorking:     true,
		WorkingIssueFormatSCM: defaultCommitTemplate,
		Queries:               DefaultQueries(),
	}
}

func (s Settings) normalized() Settings {
	if s.IssueBranchTitle == "" {
		s.IssueBranchTitle = defaultBranchTitle
	}
	if s.WorkingIssueFormatSCM == "" {
		s.WorkingIssueFormatSCM = defaultCommitTemplate
	}
	valid := make([]Query, 0, len(s.Queries))
	for _, q := range s.Queries {
		if q.Label != "" && q.Query != "" {
			q.GroupBy = slices.Clone(q.GroupBy)
			valid = append(valid, q)
		}
	}
	if len(valid) == 0 {
		valid = DefaultQueries()
	}
	s.Queries = valid
	return s
}

func (q Query) groupsByMilestone() bool {
	return slices.Contains(q.GroupBy, groupByMilestone)
}
