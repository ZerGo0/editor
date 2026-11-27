package workspace

import (
	"errors"
	"os"
	"slices"

	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui/editor"
)

const maxClosed = 30

func (w *Workspace) RunEditor(cmd editor.Command) {
	if cmd == editor.ToggleSoftWrap {
		w.ToggleWordWrap()
		return
	}
	t := w.active.ActiveTab()
	if t == nil || t.Editor == nil {
		return
	}
	t.Editor.Run(cmd)
	if w.focusedTab != t {
		w.focusTab = t
	}
}

func (w *Workspace) OpenSettings() {
	w.OpenFile(SettingsTab, false, false)
}

func (w *Workspace) OpenKeybindings() {
	w.OpenFile(KeybindingsTab, false, false)
}

const (
	emptySettings    = "{\n}\n"
	emptyKeybindings = "[\n]\n"
)

func (w *Workspace) OpenSettingsJSON() {
	w.openUserFile(w.env.Settings.Path, emptySettings)
}

func (w *Workspace) OpenKeybindingsJSON() {
	w.openUserFile(w.env.Keys.Path, emptyKeybindings)
}

func (w *Workspace) openUserFile(path, empty string) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, []byte(empty), newFilePerm); err != nil {
			w.showError("Cannot create "+path, err)
			return
		}
	}
	w.OpenFile(path, false, true)
}

func (w *Workspace) CloseAllEditors() {
	for _, g := range w.layout.Groups() {
		w.closeAll(g)
	}
}

func (w *Workspace) CloseGroup() {
	w.closeAll(w.active)
}

func (w *Workspace) CloseSaved() {
	w.closeSaved(w.active)
}

func (w *Workspace) CloseOthers() {
	if t := w.active.ActiveTab(); t != nil {
		w.closeOthers(w.active, t)
	}
}

func (w *Workspace) ReopenClosed() {
	for len(w.closed) > 0 {
		p := w.closed[len(w.closed)-1]
		w.closed = w.closed[:len(w.closed)-1]
		if _, err := os.Stat(p); err == nil || w.virtual[p] != nil {
			w.OpenFile(p, false, true)
			return
		}
	}
}

func (w *Workspace) rememberClosed(tabs []*layout.Tab) {
	for _, t := range tabs {
		w.closed = append(slices.DeleteFunc(w.closed, func(p string) bool { return p == t.Path }), t.Path)
	}
	if over := len(w.closed) - maxClosed; over > 0 {
		w.closed = w.closed[over:]
	}
}

func (w *Workspace) KeepOpen() {
	if t := w.active.ActiveTab(); t != nil {
		t.Preview = false
	}
}

func (w *Workspace) FocusExplorer() {
	w.ShowSideView(ExplorerView)
	w.tree.Focus()
}

func (w *Workspace) RevealInSidebar() {
	if t := w.active.ActiveTab(); t != nil && fsutil.Within(w.Root, t.Path) {
		w.ShowSideView(ExplorerView)
		w.tree.Reveal(t.Path)
		w.tree.Focus()
	}
}

func (w *Workspace) FocusGroup(i int) {
	groups := w.layout.Groups()
	if i >= len(groups) {
		if t := w.active.ActiveTab(); t != nil {
			w.SplitEditor(layout.Right)
		}
		return
	}
	w.active = groups[i]
	w.focusTab = w.active.ActiveTab()
}

func (w *Workspace) FocusRelativeGroup(d int) {
	groups := w.layout.Groups()
	i := slices.Index(groups, w.active)
	w.FocusGroup((i + d + len(groups)) % len(groups))
}

func (w *Workspace) ActivePath() string {
	if t := w.active.ActiveTab(); t != nil && !isVirtual(t.Path) {
		return t.Path
	}
	return ""
}

func (w *Workspace) RevealActiveInFinder() {
	if p := w.ActivePath(); p != "" {
		mygo.Shell.ShowItemInFolder(p)
	}
}

func (w *Workspace) CopyActivePath(relative bool) {
	p := w.ActivePath()
	if p == "" {
		return
	}
	if relative {
		p = w.rel(p)
	}
	mygo.Clipboard.WriteText(p)
}

func (w *Workspace) keyLabel(id string) string {
	if chords := w.env.Keys.Current.For(id); len(chords) > 0 {
		return chords[0].Label()
	}
	return ""
}
