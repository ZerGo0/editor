package workspace

import (
	"time"

	"github.com/ZerGo0/editor/internal/gitui"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	SourceControlView = "scm"
	GitPanel          = "git"
	annotateLabel     = "Annotate with Git Blame"
	confirmChoice     = 0
	cancelChoice      = 1
)

var pullModes = map[string]gitui.PullMode{
	settings.PullMerge:  gitui.PullMerge,
	settings.PullRebase: gitui.PullRebase,
	settings.PullFFOnly: gitui.PullFFOnly,
}

func (w *Workspace) initGit() {
	w.addSideView(toolView{id: SourceControlView, title: "Source Control", command: "workbench.view.scm", icon: icons.GitBranch, view: w.gitSideView, badge: w.gitChangeCount})
	w.addPanel(toolView{id: GitPanel, title: "Git", command: "git.showLog", icon: icons.Commit, view: w.gitLogPanel})
	w.statusLeft = append(w.statusLeft, w.gitStatusItems)
}

func (w *Workspace) attachGit() {
	w.git = gitui.New(w)
	w.applyGitSettings()
	w.git.OnHeadChange(w.headChanged)
}

func (w *Workspace) applyGitSettings() {
	if w.git == nil {
		return
	}
	s := w.env.Settings
	w.git.SetSettings(gitui.Settings{
		AutoFetch:   time.Duration(s.Int(settings.GitAutoFetch)) * time.Minute,
		PullMode:    pullModes[s.String(settings.GitPullMode)],
		LogPageSize: s.Int(settings.GitLogPageSize),
	})
}

func (w *Workspace) gitSideView(c *ui.Context) {
	if w.git == nil {
		loading(c)
		return
	}
	w.git.SideView(c)
}

func (w *Workspace) gitLogPanel(c *ui.Context) {
	if w.git == nil {
		loading(c)
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
	if w.git == nil || tab.Editor == nil {
		return
	}
	if base := w.git.Base(tab.Path); tab.Editor.DiffBase != base {
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
}

func selectLine(st *editor.State, line int) {
	buf := st.Buffer
	end := buf.Len()
	if line+1 < buf.LineCount() {
		end = buf.Offset(editor.Pos{Line: line + 1})
	}
	st.Select(buf.Offset(editor.Pos{Line: line}), end)
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
	if w.prs != nil {
		w.prs.BranchChanged()
	}
	if w.issues != nil {
		w.issues.BranchChanged()
	}
}

func (w *Workspace) checkoutRef(ref everywhere.GitRef) {
	if w.git == nil {
		return
	}
	if w.win == nil {
		w.git.CheckoutRef(ref.Name, ref.Remote)
		return
	}
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:       w.win,
			Type:         mygo.MessageQuestion,
			Message:      "Check out " + ref.Name + "?",
			Detail:       "The work tree switches to " + ref.Name + ". Local changes stay if they do not conflict.",
			Buttons:      []string{"Checkout", "Cancel"},
			CancelButton: cancelChoice,
		})
		if err != nil || res.Button != confirmChoice {
			return
		}
		w.update(func() { w.git.CheckoutRef(ref.Name, ref.Remote) })
	}()
}

func (w *Workspace) showCommit(oid string) {
	if w.git != nil {
		w.git.ShowCommit(oid)
	}
}
