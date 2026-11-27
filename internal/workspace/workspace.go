package workspace

import (
	"context"
	"path/filepath"
	"slices"

	"github.com/ZerGo0/editor/internal/design/pane"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/gitui"
	"github.com/ZerGo0/editor/internal/glob"
	"github.com/ZerGo0/editor/internal/history"
	"github.com/ZerGo0/editor/internal/issueui"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/palette"
	"github.com/ZerGo0/editor/internal/prefs"
	"github.com/ZerGo0/editor/internal/prui"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/ZerGo0/editor/internal/workspace/filetree"
	"github.com/ZerGo0/editor/internal/workspace/notices"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const defaultSidebarWidth = 260

var TitleBarHeight = int(token.TitleBarHeight.Px())

type Workspace struct {
	codeTheme     *editor.Theme
	Root          string
	Name          string
	win           *mygo.Window
	updates       updateQueue
	env           *Env
	docs          map[string]*docEntry
	layout        *layout.Node
	active        *layout.Group
	tree          *filetree.Tree
	sidebarWidth  float32
	sidebarHidden bool
	font          fontChoice
	nextID        int
	focusTab      *layout.Tab
	lastFocused   *layout.Tab
	focusedTab    *layout.Tab
	edited        bool
	ctx           context.Context
	cancel        context.CancelFunc
	search        searchState
	tools         toolState
	virtual       map[string]*virtualTab
	insights      insightState
	history       *history.UI
	palette       palette.Palette
	goTo          *goToLine
	prompt        prompt
	keys          keyState
	notes         notices.Store
	noteCenter    bool
	nav           navState
	focus         focusFlags
	title         titleState
	drag          dragState
	closed        []string
	settingsView  *prefs.SettingsView
	keysView      *prefs.KeysView
	ignored       ignoredState
	statusLeft    []func(c *ui.Context)
	statusHidden  map[string]bool
	indents       map[string]indentOverride
	symbols       map[string]*symbolEntry
	git           *gitui.Git
	prs           *prui.PRs
	issues        *issueui.Issues
	issueSettings issueui.Settings
	githubPanes   pane.StackState
	viewers       viewerState
	settings      *settings.Store
	folderStamp   fileStamp
	hot           hotExitState
	ec            editorConfigState
	brackets      bracketState
}

func New(root string, saved session.Project, env *Env) *Workspace {
	w := &Workspace{
		Root:          root,
		Name:          filepath.Base(root),
		env:           env,
		docs:          map[string]*docEntry{},
		statusHidden:  map[string]bool{},
		indents:       map[string]indentOverride{},
		symbols:       map[string]*symbolEntry{},
		viewers:       newViewerState(),
		sidebarWidth:  defaultSidebarWidth,
		sidebarHidden: saved.SidebarHidden,
	}
	w.settings = settings.LoadFolder(w.folderSettingsPath(), env.Settings)
	w.folderStamp, _ = statFile(w.folderSettingsPath())
	if saved.SidebarWidth > 0 {
		w.sidebarWidth = saved.SidebarWidth
	}
	expanded := make([]string, 0, len(saved.Expanded))
	for _, p := range saved.Expanded {
		expanded = append(expanded, w.abs(p))
	}
	w.tree = filetree.New(root, expanded)
	w.tree.Hidden = w.excluded
	w.settingsView = &prefs.SettingsView{Store: env.Settings, Folder: w.settings, OpenJSON: w.OpenSettingsJSON, OpenFolderJSON: w.OpenFolderSettingsJSON}
	w.keysView = &prefs.KeysView{Keys: env.Keys, Commands: env.Commands, OpenJSON: w.OpenKeybindingsJSON}
	w.initVirtual()
	w.initEditorConfig()
	w.initBrackets()
	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.initTools(saved)
	w.initInsights()
	w.initSearch(saved)
	w.initGit()
	w.initGitHub()
	w.layout = w.restoreNode(saved.Layout)
	if l := saved.LastEdit; l != nil {
		w.nav.lastEdit = &session.Location{Path: w.abs(l.Path), Line: l.Line, Col: l.Col}
	}
	groups := w.layout.Groups()
	w.active = groups[min(max(saved.ActiveGroup, 0), len(groups)-1)]
	w.focusTab = w.active.ActiveTab()
	w.initHotExit()
	return w
}

func (w *Workspace) Attach(win *mygo.Window) {
	w.win = win
	w.attachUpdates(win)
	w.loadBackups()
	go w.watch(w.ctx)
	w.reindex()
	w.refreshIgnored()
	w.attachGit()
	w.attachGitHub()
}

func (w *Workspace) Close() {
	w.cancel()
	w.stopBackupTimers()
	if w.search.timer != nil {
		w.search.timer.Stop()
	}
	if w.ignored.timer != nil {
		w.ignored.timer.Stop()
	}
	if w.git != nil {
		w.git.Close()
	}
	if w.prs != nil {
		w.prs.Close()
	}
	if w.issues != nil {
		w.issues.Close()
	}
	w.closeViewers()
}

func (w *Workspace) excluded(p string) bool {
	return glob.MatchAny(w.settings.Strings(settings.Exclude), filepath.ToSlash(w.rel(p)))
}

func (w *Workspace) SettingsChanged() {
	w.tree.InvalidateAll()
	w.scheduleReindex()
	w.applyGitSettings()
	w.applyIssueSettings()
	w.hotExitSettingsChanged()
}

func (w *Workspace) abs(p string) string {
	if filepath.IsAbs(p) || !onDisk(p) {
		return p
	}
	return filepath.Join(w.Root, p)
}

func (w *Workspace) rel(p string) string {
	r, err := filepath.Rel(w.Root, p)
	if err != nil || !filepath.IsLocal(r) || !onDisk(p) {
		return p
	}
	return r
}

func (w *Workspace) newID() int {
	w.nextID++
	return w.nextID
}

func (w *Workspace) newGroup() *layout.Group {
	return &layout.Group{ID: w.newID()}
}

func (w *Workspace) restoreNode(n *session.Node) *layout.Node {
	switch {
	case n == nil:
		return layout.Leaf(w.newGroup())
	case n.Group != nil:
		g := w.newGroup()
		for _, t := range n.Group.Tabs {
			g.Tabs = append(g.Tabs, &layout.Tab{
				ID:      w.newID(),
				Path:    w.abs(t.Path),
				Preview: t.Preview && !t.Pinned,
				Pinned:  t.Pinned,
				Restore: &layout.Position{Line: t.Line, Col: t.Col, ScrollY: t.ScrollY},
			})
		}
		g.Active = min(max(n.Group.Active, 0), max(len(g.Tabs)-1, 0))
		w.restoreNav(g, n.Group)
		return layout.Leaf(g)
	case n.First == nil:
		return w.restoreNode(n.Second)
	case n.Second == nil:
		return w.restoreNode(n.First)
	}
	return &layout.Node{Vertical: n.Vertical, Percent: n.Percent, First: w.restoreNode(n.First), Second: w.restoreNode(n.Second)}
}

func (w *Workspace) Snapshot() session.Project {
	expanded := w.tree.Expanded()
	for i, p := range expanded {
		expanded[i] = w.rel(p)
	}
	p := session.Project{
		SidebarWidth:  w.sidebarWidth,
		SidebarHidden: w.sidebarHidden,
		Expanded:      expanded,
		Layout:        w.snapshotNode(w.layout),
		ActiveGroup:   slices.Index(w.layout.Groups(), w.active),
		LastEdit:      w.lastEditSnapshot(),
	}
	w.snapshotTools(&p)
	w.snapshotSearch(&p)
	return p
}

func (w *Workspace) snapshotNode(n *layout.Node) *session.Node {
	if n.Group == nil {
		return &session.Node{Vertical: n.Vertical, Percent: n.Percent, First: w.snapshotNode(n.First), Second: w.snapshotNode(n.Second)}
	}
	g := &session.Group{Active: n.Group.Active, Tabs: make([]session.Tab, 0, len(n.Group.Tabs))}
	for _, t := range n.Group.Tabs {
		if !w.restorable(t.Path) {
			continue
		}
		st := session.Tab{Path: w.rel(t.Path), Preview: t.Preview, Pinned: t.Pinned}
		switch {
		case t.Editor != nil:
			p := t.Editor.CursorPos()
			st.Line, st.Col, st.ScrollY = p.Line, p.Col, t.Editor.ScrollY()
		case t.Restore != nil:
			st.Line, st.Col, st.ScrollY = t.Restore.Line, t.Restore.Col, t.Restore.ScrollY
		}
		g.Tabs = append(g.Tabs, st)
	}
	w.snapshotNav(n.Group, g)
	return &session.Node{Group: g}
}
