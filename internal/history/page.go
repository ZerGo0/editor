package history

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	listWidth      = 260
	rowHeight      = 40
	rowPadding     = 10
	rowRadius      = 5
	rowGap         = 2
	headerHeight   = 34
	headerPadding  = 10
	bodyFontSize   = 13
	detailFontSize = 12
	emptyPadding   = 16
	statusGap      = 8
	headerIconSize = 16
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
	split    float32

	disk     *editor.Buffer
	deleted  bool
	wantHash string
	loaded   bool
	oldText  string
	diffErr  error
	diff     *editor.DiffState
}

func newPage(path string) *page {
	p := &page{path: path, selected: -1, split: listWidth}
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
	go func() {
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
	}()
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
	go func() {
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
	}()
}

func (u *UI) pageView(c *ui.Context, p *page) {
	if p.stale {
		u.reload(p)
	}
	ui.Column(c).Fill().Background(c.Theme().Background).Children(func() {
		switch {
		case p.err != nil:
			u.failure(c, "Cannot load history", p.err, func() { u.reload(p) })
		case p.loading && len(p.versions) == 0:
			u.busy(c, "Loading history...")
		case len(p.versions) == 0:
			ui.Text(c, "No history yet. Versions are recorded each time the file is saved.").
				TextColor(c.Theme().TextMuted).Padding(emptyPadding)
		default:
			ui.Split(c, &p.split, func() { u.versionList(c, p) }, func() { u.detail(c, p) }).SplitReset(listWidth).Fill()
		}
	})
}

func (u *UI) busy(c *ui.Context, text string) {
	ui.Row(c).Gap(statusGap).Padding(emptyPadding).AlignItems(ui.Center).Children(func() {
		ui.Spinner(c).Label(text)
		ui.Text(c, text).TextColor(c.Theme().TextMuted)
	})
}

func (u *UI) failure(c *ui.Context, title string, err error, retry func()) {
	ui.Column(c).Gap(statusGap).Padding(emptyPadding).Children(func() {
		ui.Text(c, title).Bold()
		ui.Text(c, err.Error()).TextColor(c.Theme().Danger)
		if ui.Button(c, "Retry").Clicked() {
			retry()
		}
	})
}

func (u *UI) versionList(c *ui.Context, p *page) {
	t := c.Theme()
	now := u.now()
	ui.List(c, &p.list, len(p.versions)).Grow(1).FillWidth().Padding(rowGap).Gap(rowGap).Label("Versions").
		Rows(func(row ui.ListRow) {
			v := p.versions[row.Index]
			muted := t.TextMuted
			if row.Selected() && row.ListFocused() {
				muted = t.Text
			}
			rc := row.Context
			ui.Column(rc).Height(rowHeight).Padding(rowGap, rowPadding).Radius(rowRadius).Justify(ui.Center).FillWidth().Children(func() {
				ui.Row(rc).Gap(statusGap).AlignItems(ui.Center).Children(func() {
					ui.Text(rc, absoluteTime(now, v.Time)).FontSize(bodyFontSize).SingleLine().Shrink(0)
					if v.Label != "" {
						ui.Text(rc, v.Label).FontSize(bodyFontSize).Bold().TextColor(t.Accent).SingleLine().Shrink(1).MinWidth(0)
					}
				})
				ui.Text(rc, relativeTime(now, v.Time)+"  "+formatSize(v.Size)).FontSize(detailFontSize).TextColor(muted).SingleLine()
			})
		})
}

func (u *UI) detail(c *ui.Context, p *page) {
	u.syncSelection(p)
	ui.Column(c).Fill().Children(func() {
		u.toolbar(c, p)
		ui.Divider(c)
		switch {
		case p.diffErr != nil:
			u.failure(c, "Cannot read this version", p.diffErr, func() { p.wantHash = "" })
		case p.diff != nil:
			editor.DiffView(c, p.diff).Grow(1).FillWidth()
		case p.selected < 0:
			ui.Text(c, "Select a version to compare it with the current text.").TextColor(c.Theme().TextMuted).Padding(emptyPadding)
		default:
			u.busy(c, "Loading version...")
		}
	})
}

func (u *UI) toolbar(c *ui.Context, p *page) {
	t := c.Theme()
	v, ok := p.selectedVersion()
	ready := ok && p.loaded
	ui.Row(c).Height(headerHeight).Shrink(0).FillWidth().Padding(0, headerPadding).Gap(statusGap).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icons.History).Size(headerIconSize, headerIconSize).TextColor(t.TextMuted)
		title := "No version selected"
		if ok {
			title = filepath.Base(p.path) + " at " + absoluteTime(u.now(), v.Time)
		}
		ui.Text(c, title).FontSize(bodyFontSize).Bold().SingleLine().Grow(1).MinWidth(0)
		if p.deleted {
			ui.Text(c, "File deleted").FontSize(detailFontSize).TextColor(t.Warning).SingleLine().Shrink(0)
		}
		if ui.Button(c, "Revert to this version").Disabled(!ready).Clicked() {
			u.revert(p)
		}
		if ui.Button(c, "Copy").Disabled(!ready).Clicked() {
			u.copyText(p.oldText)
			u.h.Notify("Copied version text")
		}
		if ui.Button(c, "Put Label...").Tooltip("Label the current text").Clicked() {
			u.PutLabel(p.path)
		}
	})
}
