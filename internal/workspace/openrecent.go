package workspace

import (
	"path/filepath"

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
)

type recentPick struct {
	path   string
	folder bool
}

func (w *Workspace) ShowOpenRecent() {
	w.palette.Show(palette.Mode{
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
	if w.env.RecentFolders != nil {
		for _, root := range w.env.RecentFolders() {
			if root == w.Root {
				continue
			}
			items = append(items, palette.Item{
				Label:  filepath.Base(root),
				Detail: fsutil.DisplayPath(root),
				Icon:   icons.Folder,
				Group:  recentFoldersGroup,
				Value:  recentPick{path: root, folder: true},
				Action: removeAction(filepath.Base(root), func() { w.forgetRecentFolder(root) }),
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
