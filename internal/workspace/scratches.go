package workspace

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/palette"
	"github.com/ZerGo0/editor/internal/workspace/filetree"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	scratchDirName    = "scratches"
	scratchPrefix     = "scratch_"
	scratchPickTitle  = "New Scratch File"
	scratchMaxTries   = 10000
	scratchesTitle    = "Scratches"
	scratchIconSize   = 16
	scratchRowIndent  = 20
	scratchListHeight = 160
	scratchStateGap   = 4
	scratchStatePad   = 8
	scratchDetailSize = 12
	scratchIconGap    = 6
)

var errScratchName = errors.New("Enter a file name without folders.")

type scratchState struct {
	loading   bool
	loaded    bool
	err       error
	files     []string
	collapsed bool
	seq       uint64
}

type scratchPick struct {
	lang *editor.Language
}

func (w *Workspace) scratchDir() string {
	if w.env.UserData == "" {
		return ""
	}
	return filepath.Join(w.env.UserData, scratchDirName)
}

func (w *Workspace) attachScratches() {
	dir := w.scratchDir()
	if dir == "" {
		return
	}
	w.refreshScratches()
	go w.watchScratches(w.ctx, dir)
}

func (w *Workspace) watchScratches(ctx context.Context, dir string) {
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		log.Printf("scratches %s: %v", dir, err)
		return
	}
	watcher, err := mygo.WatchFiles(ctx, dir, mygo.FileWatchOptions{HintsOnly: true})
	if err != nil {
		log.Printf("watch %s: %v", dir, err)
		return
	}
	defer watcher.Close()
	for {
		if _, err := watcher.Next(ctx); err != nil {
			return
		}
		w.update(w.refreshScratches)
	}
}

func listScratches(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if e.Type().IsRegular() && !strings.HasPrefix(e.Name(), ".") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return files, nil
}

func (w *Workspace) refreshScratches() {
	dir := w.scratchDir()
	if dir == "" {
		return
	}
	w.scratches.seq++
	seq := w.scratches.seq
	w.scratches.loading = true
	apply := func(files []string, err error) {
		if seq != w.scratches.seq {
			return
		}
		w.scratches.loading, w.scratches.loaded = false, true
		w.scratches.files, w.scratches.err = files, err
	}
	if w.win == nil {
		apply(listScratches(dir))
		return
	}
	go func() {
		files, err := listScratches(dir)
		w.update(func() { apply(files, err) })
	}()
}

func (w *Workspace) NewScratchFile() {
	if w.scratchDir() == "" {
		return
	}
	items := []palette.Item{{Label: plainTextName, Detail: defaultSaveExt, Value: scratchPick{}}}
	for _, l := range editor.Languages() {
		if len(l.Extensions) > 0 {
			items = append(items, palette.Item{Label: l.Name, Detail: l.Extensions[0], Value: scratchPick{lang: l}})
		}
	}
	w.palette.Show(scratchPickTitle, palette.Tab{
		Placeholder: "Select the language of the scratch file",
		Search:      palette.Static(items),
		Hint:        "Enter to create",
	})
}

func createScratch(dir, ext string) (string, error) {
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	used := map[string]bool{}
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			used[strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))] = true
		}
	}
	for n := 1; n <= scratchMaxTries; n++ {
		stem := scratchPrefix + strconv.Itoa(n)
		if used[stem] {
			continue
		}
		p := filepath.Join(dir, stem+ext)
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, newFilePerm)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("create %s: %w", p, err)
		}
		return p, f.Close()
	}
	return "", errNoCopyName
}

func (w *Workspace) createScratch(pick scratchPick) {
	dir := w.scratchDir()
	if dir == "" {
		return
	}
	ext := extensionOf(pick.lang)
	finish := func(p string, err error) {
		if err != nil {
			w.showError("Cannot create a scratch file", err)
			return
		}
		w.refreshScratches()
		w.OpenFile(p, false, true)
	}
	if w.win == nil {
		finish(createScratch(dir, ext))
		return
	}
	go func() {
		p, err := createScratch(dir, ext)
		w.update(func() { finish(p, err) })
	}()
}

func (w *Workspace) renameScratch(p, name string) error {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return errEmptyName
	case name != filepath.Base(name) || !filepath.IsLocal(name):
		return errScratchName
	}
	to := filepath.Join(filepath.Dir(p), name)
	if to == p {
		return nil
	}
	if _, err := os.Lstat(to); err == nil {
		return fmt.Errorf("%s already exists.", name)
	}
	if err := os.Rename(p, to); err != nil {
		return err
	}
	w.moved(p, to)
	w.historyRenamed(p, to)
	w.refreshScratches()
	return nil
}

func (w *Workspace) scratchMenu(m *ui.Menu, p string) {
	if m.Item("Open").Chosen() {
		w.OpenFile(p, false, true)
	}
	m.Separator()
	if m.Item("Rename...").Chosen() {
		w.ask("Rename Scratch File", "Rename", filepath.Base(p), func(name string) error { return w.renameScratch(p, name) })
	}
	if m.Item("Delete...").Chosen() {
		w.trash(p)
	}
	m.Separator()
	if m.Item("Reveal in Finder").Chosen() {
		w.revealInFinder(p)
	}
	w.pathItems(m, p)
}

func (w *Workspace) scratchesSection(c *ui.Context) {
	if w.scratchDir() == "" {
		return
	}
	t := c.Theme()
	collapsed := w.scratches.collapsed
	ui.Row(c).Height(sectionHeight).Shrink(0).FillWidth().AlignItems(ui.Center).Padding(0, sideActionsPadding, 0, sectionPadding).Gap(sectionGap).
		BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		head := ui.ButtonBase(c.Key("scratches-head")).Grow(1).MinWidth(0).FillHeight().AlignItems(ui.Center).Justify(ui.Start).Gap(sectionGap).
			Label(scratchesTitle).Expanded(!collapsed).Radius(iconButtonRadius)
		head.Children(func() {
			chevron := icons.ChevronDown
			if collapsed {
				chevron = icons.ChevronRight
			}
			ui.Icon(c, chevron).Size(sectionChevron, sectionChevron).TextColor(t.TextMuted)
			ui.Text(c, strings.ToUpper(scratchesTitle)).FontSize(sectionFontSize).Bold().SingleLine().MinWidth(0)
		})
		head.OnClick(func() { w.scratches.collapsed = !collapsed })
		if w.tools.sideHovered {
			iconButton(c, "new-scratch", "New Scratch File...", w.withKeys("New Scratch File...", "scratch.newFile"), icons.FileAdd, false, w.NewScratchFile)
		}
	})
	if collapsed {
		return
	}
	s := &w.scratches
	switch {
	case !s.loaded:
		scratchMessage(c, func() { ui.Spinner(c).Label("Loading scratch files") })
	case s.err != nil:
		scratchMessage(c, func() {
			ui.Text(c, "Cannot load scratch files").FontSize(uiFontSize)
			ui.Text(c, s.err.Error()).FontSize(scratchDetailSize).TextColor(t.TextMuted)
			ui.Button(c.Key("scratch-retry"), "Retry").OnClick(w.refreshScratches)
		})
	case len(s.files) == 0:
		scratchMessage(c, func() {
			ui.Text(c, "No scratch files").FontSize(uiFontSize).TextColor(t.TextMuted)
			ui.Text(c, "Use New Scratch File... to make one.").FontSize(scratchDetailSize).TextColor(t.TextMuted)
		})
	default:
		ui.Scroll(c.Key("scratch-list")).FillWidth().MaxHeight(scratchListHeight).Shrink(0).Label(scratchesTitle).Children(func() {
			for _, p := range slices.Clone(s.files) {
				w.scratchRow(c, p)
			}
		})
	}
}

func scratchMessage(c *ui.Context, body func()) {
	ui.Column(c).FillWidth().Shrink(0).Gap(scratchStateGap).Padding(scratchStatePad, scratchStatePad, scratchStatePad, scratchRowIndent).Children(body)
}

func (w *Workspace) scratchRow(c *ui.Context, p string) {
	t := c.Theme()
	name := filepath.Base(p)
	row := ui.ButtonBase(c.Key("scratch-"+name)).FillWidth().Height(filetree.RowHeight).AlignItems(ui.Center).Justify(ui.Start).
		Gap(scratchIconGap).Padding(0, sectionPadding, 0, scratchRowIndent).Label(name)
	if active := w.active.ActiveTab(); active != nil && active.Path == p {
		row.Background(t.SurfacePressed)
	} else {
		pressable(row, t)
	}
	row.Children(func() {
		icons.FileIcon(c, name, false, false, scratchIconSize)
		ui.Text(c, name).FontSize(uiFontSize).SingleLine().Grow(1).MinWidth(0)
	})
	if row.DoubleClicked() {
		w.OpenFile(p, false, true)
	} else {
		row.OnClick(func() { w.OpenFile(p, true, false) })
	}
	row.ContextMenu(func(m *ui.Menu) { w.scratchMenu(m, p) })
}
