package history

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/detail"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/pane"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	listWidth     = 260
	revertLabel   = "Revert to This Version"
	emptyMessage  = "Versions are recorded each time the file is saved."
	selectMessage = "Select a version to compare it with the current text."
)

type page struct {
	path     string
	stale    bool
	gen      int
	loading  bool
	err      error
	versions []Version
	list     ui.ListState
	selected int

	disk     *editor.Buffer
	deleted  bool
	wantHash string
	loaded   bool
	oldText  string
	diffErr  error
	diff     *editor.DiffState
}

func newPage(path string) *page {
	p := &page{path: path, selected: -1}
	p.list.Selected = &p.selected
	p.list.Key = func(i int) any { return p.versions[i].Hash + p.versions[i].Time.String() }
	p.list.Label = func(i int) string {
		v := p.versions[i]
		return strings.TrimSpace(v.Time.Local().Format(absoluteFmt) + " " + v.Label)
	}
	return p
}

func (p *page) selectedVersion() (Version, bool) {
	if p.selected < 0 || p.selected >= len(p.versions) {
		return Version{}, false
	}
	return p.versions[p.selected], true
}

type pageData struct {
	versions []Version
	disk     string
	deleted  bool
}

func (u *UI) reload(p *page) {
	p.gen++
	p.loading, p.err, p.stale = true, nil, false
	gen, root, path := p.gen, u.h.RootDir(), p.path
	open := u.h.Buffer(path) != nil
	applog.Go(func() {
		var d pageData
		vs, err := u.store.Versions(root, path)
		d.versions = vs
		if err == nil && !open {
			var content []byte
			content, err = os.ReadFile(path)
			if errors.Is(err, fs.ErrNotExist) {
				d.deleted, err = true, nil
			}
			d.disk = string(content)
		}
		u.h.Update(func() {
			if gen == p.gen {
				u.loadedPage(p, d, err)
			}
		})
	})
}

func (u *UI) loadedPage(p *page, d pageData, err error) {
	p.loading = false
	if err != nil {
		p.err = err
		return
	}
	var keep string
	if v, ok := p.selectedVersion(); ok {
		keep = v.Hash
	}
	p.versions, p.deleted = d.versions, d.deleted
	p.disk = editor.NewBuffer(d.disk)
	p.disk.SetLanguage(u.h.Language(p.path, d.disk))
	p.selected = -1
	for i, v := range p.versions {
		if v.Hash == keep {
			p.selected = i
		}
	}
	if p.selected < 0 && len(p.versions) > 0 {
		p.selected = 0
	}
	if v, ok := p.selectedVersion(); !ok || v.Hash != keep {
		p.wantHash, p.loaded, p.diff = "", false, nil
	}
}

func (u *UI) current(p *page) *editor.Buffer {
	if buf := u.h.Buffer(p.path); buf != nil {
		return buf
	}
	return p.disk
}

func (u *UI) syncSelection(p *page) {
	v, ok := p.selectedVersion()
	if !ok {
		return
	}
	if v.Hash != p.wantHash {
		p.wantHash, p.loaded, p.diff, p.diffErr = v.Hash, false, nil, nil
		u.loadVersion(p, v.Hash)
	}
	if cur := u.current(p); p.loaded && cur != nil && (p.diff == nil || p.diff.New != cur) {
		old := editor.NewBuffer(p.oldText)
		old.SetLanguage(u.h.Language(p.path, p.oldText))
		p.diff = &editor.DiffState{Old: old, New: cur}
	}
}

func (u *UI) loadVersion(p *page, hash string) {
	applog.Go(func() {
		data, err := u.store.Read(hash)
		u.h.Update(func() {
			if p.wantHash != hash {
				return
			}
			if err != nil {
				p.diffErr = err
				return
			}
			p.oldText, p.loaded = string(data), true
		})
	})
}

func (u *UI) pageView(c *ui.Context, p *page) {
	if p.stale {
		u.reload(p)
	}
	pal := token.Of(c)
	ui.Column(c).Fill().Background(pal.Layer.Editor).Children(func() {
		switch {
		case p.err != nil:
			detail.Failed(c, "Cannot load history", p.err, func() { u.reload(p) })
		case p.loading && len(p.versions) == 0:
			detail.Loading(c, "Loading history...")
		case len(p.versions) == 0:
			detail.Centered(c, func() {
				feedback.Empty(c, feedback.EmptyOptions{Icon: icons.History, Title: "No history yet", Message: emptyMessage})
			})
		default:
			ui.Split(c, &u.split, func() { u.versionList(c, p) }, func() { u.detail(c, p) }).SplitReset(listWidth).Fill()
		}
	})
}

func (u *UI) versionList(c *ui.Context, p *page) {
	pal := token.Of(c)
	now := u.now()
	ui.Column(c).Fill().Background(pal.Layer.SideBar).Children(func() {
		pane.ViewTitle(c, "Versions", pane.TitleOptions{})
		list.View(c, &p.list, len(p.versions), list.ViewOptions{Label: "Versions"}, func(i int) list.Item {
			v := p.versions[i]
			it := list.Item{
				Label:       absoluteTime(now, v.Time),
				Description: relativeTime(now, v.Time) + "  " + formatSize(v.Size),
				Icon:        icons.History,
			}
			if v.Label != "" {
				it.Trailing = func(c *ui.Context) { badge.Tag(c, v.Label, badge.TagOptions{Tone: token.ToneInfo}) }
			}
			return it
		})
	})
}

func (u *UI) detail(c *ui.Context, p *page) {
	u.syncSelection(p)
	ui.Column(c).Fill().Children(func() {
		u.toolbar(c, p)
		switch {
		case p.diffErr != nil:
			detail.Failed(c, "Cannot read this version", p.diffErr, func() { p.wantHash = "" })
		case p.diff != nil:
			u.h.ConfigureDiff(p.diff, p.path)
			editor.DiffView(c, p.diff).Grow(1).FillWidth()
		case p.selected < 0:
			detail.Centered(c, func() {
				feedback.Empty(c, feedback.EmptyOptions{Title: "Select a version", Message: selectMessage})
			})
		default:
			detail.Loading(c, "Loading version...")
		}
	})
}

func (u *UI) toolbar(c *ui.Context, p *page) {
	pal := token.Of(c)
	v, ok := p.selectedVersion()
	ready := ok && p.loaded
	ui.Row(c).Height(token.ViewTitleHeight.Px()).Shrink(0).FillWidth().Padding(0, token.PadRow.Px(), 0, token.PadView.Px()).Gap(token.GapControl.Px()).
		AlignItems(ui.Center).Background(pal.Layer.Panel).BorderWidth(0, 0, token.BorderWidth.Px(), 0).BorderColor(pal.Border.Default).
		Role(ui.RoleToolbar).Label("Version actions").Children(func() {
		title := "No version selected"
		if ok {
			title = filepath.Base(p.path) + " at " + absoluteTime(u.now(), v.Time)
		}
		design.Text(c, title, token.TextStrong).SingleLine().Grow(1).MinWidth(0)
		if p.deleted {
			badge.Tag(c, "File deleted", badge.TagOptions{Tone: token.ToneWarning})
		}
		if action.Button(c, revertLabel, action.ButtonOptions{Small: true, Disabled: !ready}).Clicked() {
			u.revert(p)
		}
		if action.Button(c, "Copy", action.ButtonOptions{Variant: action.Ghost, Small: true, Disabled: !ready}).Clicked() {
			u.copyText(p.oldText)
			u.h.Notify("Copied the version text")
		}
		if action.Button(c, "Put Label...", action.ButtonOptions{Variant: action.Ghost, Small: true, Tooltip: "Label the Current Text"}).Clicked() {
			u.PutLabel(p.path)
		}
	})
}
