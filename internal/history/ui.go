package history

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/host"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	ViewPrefix        = "history/"
	ViewTitlePrefix   = "History: "
	ActionShow        = "history.show"
	ActionPutLabel    = "history.putLabel"
	categoryHistory   = "History"
	beforeRevertLabel = "Before revert"
	revertedLabel     = "Reverted"
	defaultFilePerm   = fs.FileMode(0o644)
	splitStateKey     = "history.split"
)

type Action struct {
	ID       string
	Title    string
	Category string
	Key      string
}

var Actions = []Action{
	{ID: ActionShow, Title: "Show History", Category: categoryHistory},
	{ID: ActionPutLabel, Title: "Put Label...", Category: categoryHistory},
}

var errEmptyLabel = errors.New("Enter a label.")

type UI struct {
	h        host.Host
	store    *Store
	pages    map[string]*page
	now      func() time.Time
	confirm  func(parent *mygo.Window, name string, at time.Time) bool
	copyText func(text string)
	split    float32

	lowDiskNotified bool
}

func NewUI(h host.Host, s *Store) *UI {
	u := &UI{
		h:        h,
		store:    s,
		pages:    map[string]*page{},
		now:      time.Now,
		confirm:  confirmRevert,
		copyText: mygo.Clipboard.WriteText,
		split:    listWidth,
	}
	state := h.ProjectState()
	if !state.Load(splitStateKey, &u.split) || u.split <= 0 {
		u.split = listWidth
	}
	state.Track(splitStateKey, func() any { return u.split })
	return u
}

func confirmRevert(parent *mygo.Window, name string, at time.Time) bool {
	res, err := mygo.Dialog.Message(mygo.MessageOptions{
		Parent:             parent,
		Type:               mygo.MessageWarning,
		Message:            fmt.Sprintf("Revert %q to the version from %s?", name, at.Local().Format(absoluteFmt)),
		Detail:             "The current text stays in Local History.",
		Buttons:            []string{"Revert", "Cancel"},
		CancelButton:       1,
		DestructiveButtons: []int{0},
	})
	return err == nil && res.Button == 0
}

func (u *UI) Record(path string, content []byte, label string) {
	u.recordThen(path, bytes.Clone(content), label, u.reportAuto)
}

func (u *UI) recordThen(path string, content []byte, label string, report func(error)) {
	root := u.h.RootDir()
	applog.Go(func() {
		err := u.store.Record(root, path, content, label)
		u.h.Update(func() {
			if err != nil {
				report(err)
			}
			u.markStale(path)
		})
	})
}

func (u *UI) reportAuto(err error) {
	switch {
	case errors.Is(err, ErrTooLarge):
	case errors.Is(err, ErrLowDisk):
		if !u.lowDiskNotified {
			u.lowDiskNotified = true
			u.h.Notify("Local History paused: low disk space.")
		}
	default:
		u.h.ShowError("Local History cannot save a version", err)
	}
}

func (u *UI) markStale(path string) {
	if p := u.pages[path]; p != nil {
		p.stale = true
	}
}

func (u *UI) Renamed(from, to string) {
	root := u.h.RootDir()
	for path, p := range u.pages {
		if moved, ok := remapPath(path, from, to); ok {
			delete(u.pages, path)
			p.path = moved
			u.pages[moved] = p
		}
	}
	applog.Go(func() {
		if err := u.store.Renamed(root, from, to); err != nil {
			u.h.Update(func() { u.reportAuto(err) })
		}
	})
}

func (u *UI) Run(id string) bool {
	switch id {
	case ActionShow:
		u.withActive(u.ShowHistory)
	case ActionPutLabel:
		u.withActive(u.PutLabel)
	default:
		return false
	}
	return true
}

func (u *UI) withActive(fn func(path string)) {
	path := u.h.ActivePath()
	if path == "" {
		u.h.Notify("Open a file first.")
		return
	}
	fn(path)
}

func (u *UI) ShowHistory(path string) {
	p := u.pages[path]
	if p == nil {
		p = newPage(path)
		u.pages[path] = p
	}
	u.reload(p)
	u.h.OpenView(ViewPrefix+path, ViewTitlePrefix+filepath.Base(path), func(c *ui.Context) { u.pageView(c, p) })
}

func (u *UI) PutLabel(path string) {
	u.h.Ask("Put Label", "Put Label", "", func(label string) error {
		if label == "" {
			return errEmptyLabel
		}
		report := func(err error) { u.h.ShowError("Cannot put label", err) }
		if buf := u.h.Buffer(path); buf != nil {
			u.recordThen(path, []byte(buf.String()), label, report)
			return nil
		}
		root := u.h.RootDir()
		applog.Go(func() {
			content, err := os.ReadFile(path)
			if err == nil {
				err = u.store.Record(root, path, content, label)
			}
			u.h.Update(func() {
				if err != nil {
					report(err)
					return
				}
				u.markStale(path)
			})
		})
		return nil
	})
}

func (u *UI) revert(p *page) {
	v, ok := p.selectedVersion()
	if !ok || !p.loaded {
		return
	}
	path, text, parent := p.path, p.oldText, u.h.Window()
	applog.Go(func() {
		if !u.confirm(parent, filepath.Base(path), v.Time) {
			return
		}
		u.h.Update(func() { u.applyRevert(path, text) })
	})
}

func (u *UI) applyRevert(path, text string) {
	if buf := u.h.Buffer(path); buf != nil {
		u.recordThen(path, []byte(buf.String()), beforeRevertLabel, u.reportAuto)
		buf.Set(text)
		u.h.Notify("Reverted " + filepath.Base(path))
		return
	}
	root := u.h.RootDir()
	applog.Go(func() {
		err := u.revertOnDisk(root, path, text)
		u.h.Update(func() {
			if err != nil {
				u.h.ShowError("Cannot revert "+filepath.Base(path), err)
				return
			}
			u.markStale(path)
			u.h.Notify("Reverted " + filepath.Base(path))
		})
	})
}

func (u *UI) revertOnDisk(root, path, text string) error {
	perm := defaultFilePerm
	if st, err := os.Stat(path); err == nil {
		perm = st.Mode().Perm()
	}
	if cur, err := os.ReadFile(path); err == nil {
		_ = u.store.Record(root, path, cur, beforeRevertLabel)
	}
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(path, []byte(text), perm); err != nil {
		return err
	}
	_ = u.store.Record(root, path, []byte(text), revertedLabel)
	return nil
}
