package workspace

import (
	"encoding/json"
	"reflect"

	"github.com/ZerGo0/editor/internal/design/pane"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/insights"
	"github.com/ZerGo0/editor/internal/issueui"
	"github.com/ZerGo0/editor/internal/prui"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

var githubSections = []string{sectionPullRequests, sectionIssues, sectionNotifications}

const (
	GitHubView           = "github"
	sectionPullRequests  = "pull-requests"
	sectionIssues        = "issues"
	sectionNotifications = "notifications"
	queriesSettingError  = "Cannot read " + settings.IssueQueries + ": "
	markdownLanguage     = "Markdown"
	issueTrigger         = "#"
	mentionTrigger       = "@"
)

func (w *Workspace) initGitHub() {
	w.addSideView(toolView{id: GitHubView, title: "GitHub", command: "workbench.view.github", icon: icons.Github, view: w.githubView, badge: w.githubBadge})
	w.statusLeft = append(w.statusLeft, w.githubStatusItems)
	w.insights.todoList.EntryMenu = w.todoMenu
}

func (w *Workspace) attachGitHub() {
	w.prs = prui.New(w)
	w.issues = issueui.New(w)
	w.applyIssueSettings()
	if w.git != nil {
		w.git.SetCommitTemplate(w.issues.CommitTemplate)
	}
}

func (w *Workspace) applyIssueSettings() {
	if w.issues == nil {
		return
	}
	st := w.settings
	s := issueui.DefaultSettings()
	if v := st.String(settings.IssueBranchTitle); v != "" {
		s.IssueBranchTitle = v
	}
	if v := st.String(settings.IssueCommitFormat); v != "" {
		s.WorkingIssueFormatSCM = v
	}
	s.AssignWhenWorking = st.Bool(settings.IssueAssignWorking)
	s.LinkBranchOnGitHub = st.Bool(settings.IssueLinkBranch)
	if raw := st.JSON(settings.IssueQueries); len(raw) > 0 {
		var queries []issueui.Query
		if err := json.Unmarshal(raw, &queries); err != nil {
			w.setNotice(queriesSettingError + err.Error())
		} else {
			s.Queries = queries
		}
	}
	if reflect.DeepEqual(s, w.issueSettings) {
		return
	}
	w.issueSettings = s
	w.issues.SetSettings(s)
}

func (w *Workspace) githubBadge() int {
	if w.prs == nil {
		return 0
	}
	return w.prs.Badge() + w.prs.NotificationsBadge()
}

func (w *Workspace) githubStatusItems(c *ui.Context) {
	if w.prs != nil {
		w.prs.StatusItems(c)
	}
	if w.issues != nil {
		w.issues.StatusItems(c)
	}
}

func (w *Workspace) githubView(c *ui.Context) {
	if w.prs == nil || w.issues == nil {
		loading(c)
		return
	}
	if w.prs.StateView(c) {
		return
	}
	if w.githubPanes.Collapsed == nil {
		w.githubPanes.Collapsed = map[string]bool{}
		for _, id := range githubSections {
			w.githubPanes.Collapsed[id] = w.tools.collapsed[id]
		}
	}
	pane.Stack(c, &w.githubPanes, []pane.Pane{
		{ID: sectionPullRequests, Title: w.prs.Title(), Count: w.prs.Badge(), Actions: w.prs.SideActions, Body: w.prs.SideView},
		{ID: sectionIssues, Title: w.issues.Title(), Count: w.issues.Badge(), Actions: w.issues.SideActions, Body: w.issues.SideView},
		{ID: sectionNotifications, Title: w.prs.NotificationsTitle(), Count: w.prs.NotificationsBadge(), Actions: w.prs.NotificationsActions,
			Busy: w.prs.NotificationsLoading(), Body: w.prs.NotificationsView},
	})
	for _, id := range githubSections {
		w.tools.collapsed[id] = w.githubPanes.Collapsed[id]
	}
}

func (w *Workspace) RunPullRequests(id string) {
	if w.prs == nil {
		return
	}
	if id == prui.ActionSearch {
		w.ShowSideView(GitHubView)
		w.tools.collapsed[sectionPullRequests] = false
		if w.githubPanes.Collapsed != nil {
			w.githubPanes.Collapsed[sectionPullRequests] = false
		}
	}
	w.prs.Run(id)
}

func (w *Workspace) RunIssues(id string) {
	if w.issues != nil {
		w.issues.Run(id)
	}
}

func (w *Workspace) todoMenu(m *ui.Menu, path string, e insights.Entry) {
	if w.issues == nil {
		return
	}
	m.Separator()
	if m.Item("Create Issue from TODO").Chosen() {
		w.issues.CreateFromTODO(path, e.Line, e.Detail)
	}
}

func (w *Workspace) githubEditorItems(m *ui.Menu) {
	if w.issues == nil || w.ActivePath() == "" {
		return
	}
	m.Separator()
	w.commandItem(m, "Copy GitHub Permalink", issueui.ActionCopyPermalink)
	w.commandItem(m, "Open on GitHub", issueui.ActionOpenPermalink)
	w.commandItem(m, "Create Issue from Selection", issueui.ActionCreateFromSelection)
}

func (w *Workspace) markdownCompletions(st *editor.State) func(editor.CompletionRequest) {
	return func(req editor.CompletionRequest) {
		start := req.Offset - len(req.Prefix)
		if w.issues == nil || start <= 0 {
			return
		}
		trigger := st.Buffer.Slice(start-1, start)
		if trigger != issueTrigger && trigger != mentionTrigger {
			return
		}
		if items := w.issues.Completions(trigger + req.Prefix); len(items) > 0 {
			st.ShowCompletions(start-1, items)
		}
	}
}

func (w *Workspace) attachCompletions(st *editor.State) {
	if lang := st.Buffer.Language(); lang != nil && lang.Name == markdownLanguage {
		st.OnCompletion = w.markdownCompletions(st)
	}
}
