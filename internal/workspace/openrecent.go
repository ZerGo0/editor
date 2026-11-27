package workspace

import (
	"path/filepath"
	"slices"

	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/palette"
	"github.com/egoist/mygo/ui"
)

const (
	openRecentTitle       = "Open Recent"
	openRecentPlaceholder = "Select a folder or file to open"
	openRecentEmpty       = "No recent folders or files"
	recentFoldersGroup    = "folders"
	recentFilesGroup      = "files"
	openedHereLabel       = "Folder Opened in Active Window"
	openedElsewhereLabel  = "Folder Opened in a Window"
)

type recentPick struct {
	path   string
	folder bool
}

func (w *Workspace) ShowOpenRecent() {
	selected := 0
	if items := w.recentEntries(); len(items) > 1 && items[0].Value == (recentPick{path: w.Root, folder: true}) {
		selected = 1
	}
	w.palette.Show(palette.Mode{
		Selected:    selected,
		Label:       openRecentTitle,
		Placeholder: openRecentPlaceholder,
		Empty:       openRecentEmpty,
		Search:      func(query string, done func([]palette.Item)) { done(palette.Filter(w.recentEntries(), query)) },
		Accept:      func(it palette.Item) { w.openRecent(it.Value.(recentPick), true) },
		AcceptAlt:   func(it palette.Item) { w.openRecent(it.Value.(recentPick), false) },
	})
}

func (w *Workspace) recentEntries() []palette.Item {
	var items []palette.Item
	var open []string
	if w.env.OpenFolders != nil {
		open = w.env.OpenFolders()
	}
	if w.env.RecentFolders != nil {
		for _, root := range w.env.RecentFolders() {
			items = append(items, palette.Item{
				Label:  filepath.Base(root),
				Detail: fsutil.DisplayPath(root),
				Icon:   icons.Folder,
				Group:  recentFoldersGroup,
				Value:  recentPick{path: root, folder: true},
				Action: w.folderAction(root, open),
			})
		}
	}
	for _, path := range w.search.recent {
		item := palette.Item{
			Label:   filepath.Base(path),
			Leading: func(c *ui.Context) { w.tabIcon(c, path) },
			Group:   recentFilesGroup,
			Value:   recentPick{path: path},
			Action:  removeAction(filepath.Base(path), func() { w.forgetRecent(path) }),
		}
		if dir := w.rel(filepath.Dir(path)); dir != "." {
			item.Detail = fsutil.DisplayPath(dir)
		}
		items = append(items, item)
	}
	return items
}

func removeAction(name string, run func()) *palette.Action {
	return &palette.Action{Icon: icons.Close, Label: "Remove " + name + " from Recently Opened", Run: run}
}

func (w *Workspace) folderAction(root string, open []string) *palette.Action {
	forget := func() { w.forgetRecentFolder(root) }
	switch {
	case root == w.Root:
		return &palette.Action{Icon: icons.FolderOpen, Label: openedHereLabel, Run: forget}
	case slices.Contains(open, root):
		return &palette.Action{Icon: icons.FolderOpen, Label: openedElsewhereLabel, Run: forget}
	}
	return removeAction(filepath.Base(root), forget)
}

func (w *Workspace) forgetRecentFolder(root string) {
	if w.env.ForgetFolder != nil {
		w.env.ForgetFolder(root)
	}
}

func (w *Workspace) openRecent(p recentPick, sameWindow bool) {
	switch {
	case !p.folder:
		w.OpenFile(p.path, false, true)
	case w.env.OpenFolder != nil:
		w.env.OpenFolder(w.win, p.path, sameWindow)
	}
}
