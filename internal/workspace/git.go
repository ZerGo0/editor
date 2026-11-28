package workspace

import (
	"errors"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/design/detail"
	"github.com/ZerGo0/editor/internal/gitui"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/plural"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	SourceControlView = "scm"
	GitPanel          = "git"
	annotateLabel     = "Annotate with Git Blame"
)

var errNoFolderPicker = errors.New("no folder picker in this window")

var pullModes = map[string]gitui.PullMode{
	settings.PullMerge:  gitui.PullMerge,
	settings.PullRebase: gitui.PullRebase,
	settings.PullFFOnly: gitui.PullFFOnly,
}

func (w *Workspace) initGit() {
	w.addSideView(toolView{id: SourceControlView, title: "Source Control", command: "workbench.view.scm", icon: icons.GitBranch, view: w.gitSideView, badge: w.gitChangeCount, badgeTip: pendingChangesTip, actions: w.gitTitleActions, focus: w.focusCommitMessage})
	w.addPanel(toolView{id: GitPanel, title: "Git", command: "git.showLog", icon: icons.Commit, view: w.gitLogPanel})
	w.statusLeft = append(w.statusLeft, w.gitStatusItems)
}

func (w *Workspace) attachGit() {
	w.git = gitui.New(w)
	w.applyGitSettings()
	w.git.OnHeadChange(w.headChanged)
	w.git.OnStatus(w.tree.SetGit)
	w.git.OnToggleWrap(w.ToggleWordWrap)
}

func (w *Workspace) applyGitSettings() {
	if w.git == nil {
		return
	}
	s := w.settings
	w.git.SetSettings(gitui.Settings{
		AutoFetch:   time.Duration(s.Int(settings.GitAutoFetch)) * time.Minute,
		PullMode:    pullModes[s.String(settings.GitPullMode)],
		LogPageSize: s.Int(settings.GitLogPageSize),
		Unified:     !s.Bool(settings.DiffSideBySide),
	})
}

func (w *Workspace) PickFolder(title string) (string, error) {
	if w.env.PickFolder == nil {
		return "", errNoFolderPicker
	}
	paths, err := w.env.PickFolder(w.win, title)
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

func (w *Workspace) OpenFolder(root string) {
	if w.env.OpenFolder != nil {
		w.env.OpenFolder(w.win, root, false)
	}
}

func (w *Workspace) focusCommitMessage() bool {
	return w.git != nil && w.git.FocusCommitMessage()
}

func pendingChangesTip(n int) string {
	return plural.Count(n, "pending change", "pending changes")
}

func (w *Workspace) gitSideView(c *ui.Context) {
	if w.git == nil {
		detail.Loading(c, loadingLabel)
		return
	}
	w.git.SideView(c)
}

func (w *Workspace) gitTitleActions(c *ui.Context) {
	if w.git != nil {
		w.git.TitleActions(c)
	}
}

func (w *Workspace) ActiveViewID() string {
	if t := w.active.ActiveTab(); t != nil && isVirtual(t.Path) {
		return strings.TrimPrefix(t.Path, virtualPrefix)
	}
	return ""
}

func (w *Workspace) diffContext() (active, focused bool) {
	if w.git == nil {
		return false, false
	}
	return w.git.DiffContext()
}

func (w *Workspace) SourceControlFocused() bool {
	return !w.sidebarHidden && w.focus.side && w.tools.side == SourceControlView
}

func (w *Workspace) OpenViewWith(id, title string, view func(c *ui.Context), preview, focus bool) {
	p := virtualPrefix + id
	w.virtual[p] = &virtualTab{title: title, view: view}
	w.OpenFile(p, preview, focus)
}

func (w *Workspace) gitLogPanel(c *ui.Context) {
	if w.git == nil {
		detail.Loading(c, loadingLabel)
		return
	}
	w.git.LogPanel(c)
}

func (w *Workspace) gitChangeCount() int {
	if w.git == nil {
		return 0
	}
	return w.git.ChangeCount()
}

func (w *Workspace) gitStatusItems(c *ui.Context) {
	if w.git != nil {
		w.git.StatusItems(c)
	}
}

func (w *Workspace) RunGit(id string) {
	if w.git == nil {
		return
	}
	w.git.Run(id)
	if id == "git.commit" {
		w.ShowSideView(SourceControlView)
	}
}

func (w *Workspace) gitFrame() {
	if w.git != nil && w.git.WantsPanel() {
		w.ShowPanel(GitPanel)
	}
}

func (w *Workspace) applyDiffBase(tab *layout.Tab) {
	if w.git == nil {
		return
	}
	base := w.git.Base(tab.Path)
	if tab.Editor == nil {
		return
	}
	if base == nil {
		base = tab.Editor.Buffer
	}
	if tab.Editor.DiffBase != base {
		tab.Editor.DiffBase = base
	}
}

func (w *Workspace) gutterClick(tab *layout.Tab, line int) {
	st := tab.Editor
	if st == nil {
		return
	}
	if g := w.groupOf(tab); g != nil {
		w.activate(g, tab)
	}
	if w.git != nil && w.git.HasChangeAt(tab.Path, st.Buffer, line) {
		st.GoTo(editor.Pos{Line: line})
		w.git.Run("git.showChange")
		return
	}
	selectLine(st, line)
	w.gutter = &gutterDrag{st: st, line: line}
}

func selectLine(st *editor.State, line int) {
	st.Select(lineOffset(st.Buffer, line), lineOffset(st.Buffer, line+1))
}

func (w *Workspace) groupOf(tab *layout.Tab) *layout.Group {
	for _, g := range w.layout.Groups() {
		if g.Index(tab) >= 0 {
			return g
		}
	}
	return nil
}

func (w *Workspace) gitEditorItems(m *ui.Menu) {
	path := w.ActivePath()
	if w.git == nil || path == "" {
		return
	}
	m.Separator()
	if m.Item(annotateLabel).Checked(w.git.Annotated(path)).Chosen() {
		w.RunGit("git.annotate")
	}
	w.commandItem(m, "Show History for Current File", "git.fileHistory")
	w.commandItem(m, "Show Diff with HEAD", "git.showDiff")
	w.commandItem(m, "Rollback Changed Lines", "git.rollbackLines")
}

func (w *Workspace) headChanged() {
	w.scheduleIgnored()
	if w.prs != nil {
		w.prs.BranchChanged()
	}
	if w.issues != nil {
		w.issues.BranchChanged()
	}
}
