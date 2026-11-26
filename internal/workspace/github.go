package workspace

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/insights"
	"github.com/ZerGo0/editor/internal/issueui"
	"github.com/ZerGo0/editor/internal/prui"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

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

type viewSection struct {
	id, title string
	view      func(c *ui.Context)
	badge     func() int
}

func (w *Workspace) initGitHub() {
	w.addSideView(toolView{id: GitHubView, title: "GitHub", command: "workbench.view.github", icon: icons.PullRequest, view: w.githubView, badge: w.githubBadge})
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
	w.viewSections(c, []viewSection{
		{id: sectionPullRequests, title: w.prs.Title(), view: w.prs.SideView, badge: w.prs.Badge},
		{id: sectionIssues, title: w.issues.Title(), view: w.issues.SideView, badge: w.issues.Badge},
		{id: sectionNotifications, title: w.prs.NotificationsTitle(), view: w.prs.NotificationsView, badge: w.prs.NotificationsBadge},
	})
}

func (w *Workspace) viewSections(c *ui.Context, sections []viewSection) {
	open := 0
	for i, s := range sections {
		w.sectionHeader(c, s, i > 0)
		if w.tools.collapsed[s.id] {
			continue
		}
		open++
		ui.Column(c.Key("section-" + s.id)).Grow(1).FillWidth().MinHeight(0).Children(func() { s.view(c) })
	}
	if open == 0 {
		ui.Box(c).Grow(1)
	}
}

func (w *Workspace) sectionHeader(c *ui.Context, s viewSection, border bool) {
	t := c.Theme()
	collapsed := w.tools.collapsed[s.id]
	row := ui.Row(c).Height(sectionHeight).Shrink(0).FillWidth().AlignItems(ui.Center).Padding(0, sideActionsPadding, 0, sectionPadding)
	if border {
		row.BorderWidth(1, 0, 0, 0).BorderColor(t.Border)
	}
	row.Children(func() {
		head := ui.ButtonBase(c.Key("section-head-" + s.id)).Grow(1).MinWidth(0).FillHeight().AlignItems(ui.Center).Justify(ui.Start).Gap(sectionGap).
			Label(s.title).Expanded(!collapsed).Radius(iconButtonRadius)
		head.Children(func() {
			chevron := icons.ChevronDown
			if collapsed {
				chevron = icons.ChevronRight
			}
			ui.Icon(c, chevron).Size(sectionChevron, sectionChevron).TextColor(t.TextMuted)
			ui.Text(c, strings.ToUpper(s.title)).FontSize(sectionFontSize).Bold().SingleLine().MinWidth(0).Shrink(1)
			ui.Box(c).Grow(1)
			if s.badge != nil {
				countBadge(c, s.badge())
			}
		})
		head.OnClick(func() { w.tools.collapsed[s.id] = !collapsed })
	})
}

func (w *Workspace) RunPullRequests(id string) {
	if w.prs == nil {
		return
	}
	if id == prui.ActionSearch {
		w.ShowSideView(GitHubView)
		w.tools.collapsed[sectionPullRequests] = false
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
