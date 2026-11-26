package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"log"
	"path/filepath"
	"slices"
	"time"

	"github.com/ZerGo0/editor/internal/backup"
	"github.com/ZerGo0/editor/internal/document"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/ZerGo0/editor/internal/syntax"
	"github.com/ZerGo0/editor/internal/viewer"
)

const (
	backupDelay     = time.Second
	projectKeyBytes = 8
)

type hotExitState struct {
	store   *backup.Store
	seq     uint64
	docs    map[string]*backupDoc
	pending map[string]backup.Meta
}

type backupDoc struct {
	timer     *time.Timer
	scheduled uint64
	known     bool
	written   uint64
	stored    bool
	tooLarge  bool
	removed   bool
}

func projectKey(root string) string {
	sum := sha256.Sum256([]byte(root))
	return hex.EncodeToString(sum[:projectKeyBytes])
}

func (w *Workspace) initHotExit() {
	w.hot.docs = map[string]*backupDoc{}
	w.hot.pending = map[string]backup.Meta{}
	if w.env.BackupDir == "" {
		return
	}
	w.hot.store = backup.Open(filepath.Join(w.env.BackupDir, projectKey(w.Root)), backup.DefaultLimit)
	if !w.hotExitOn() {
		w.clearBackups()
		return
	}
	open := map[string]bool{}
	for _, t := range w.tabs() {
		open[t.Path] = true
	}
	g := w.active
	active := g.Active
	for _, m := range w.hot.store.Entries() {
		w.hot.pending[m.Path] = m
		if !open[m.Path] {
			g.Open(m.Path, false, w.newID())
		}
	}
	if active < len(g.Tabs) && len(open) > 0 {
		g.Active = active
	}
}

func (w *Workspace) hotExitOn() bool {
	return w.hot.store != nil && w.settings.String(settings.HotExit) == settings.HotExitOnClose
}

func (w *Workspace) hasBackup(path string) bool {
	return w.hot.store != nil && w.hot.store.Has(path)
}

func (w *Workspace) backupState(path string) *backupDoc {
	b := w.hot.docs[path]
	if b == nil || b.removed {
		b = &backupDoc{}
		w.hot.docs[path] = b
	}
	return b
}

func backupable(d *document.Document) bool {
	return d != nil && d.Buffer != nil && d.Large == nil && !d.ReadOnly
}

func (w *Workspace) syncBackups() {
	if !w.hotExitOn() {
		return
	}
	for path, e := range w.docs {
		d := e.doc
		if e.loading || !backupable(d) {
			continue
		}
		if !d.Modified() {
			if b := w.hot.docs[path]; b != nil && !b.removed || b == nil && w.hot.store.Has(path) {
				w.deleteBackup(path)
			}
			continue
		}
		b := w.backupState(path)
		v := d.Buffer.Version()
		if b.known && b.scheduled == v {
			continue
		}
		b.scheduled, b.known = v, true
		if b.stored && b.written == v {
			continue
		}
		if b.timer != nil {
			b.timer.Reset(backupDelay)
			continue
		}
		b.timer = time.AfterFunc(backupDelay, func() { w.update(func() { w.backupDoc(path) }) })
	}
}

func (w *Workspace) backupMeta(d *document.Document) backup.Meta {
	m := backup.Meta{Path: d.Path}
	m.DiskSize, m.DiskModTime = d.DiskStamp()
	if l := d.Buffer.Language(); l != nil && isUntitled(d.Path) {
		m.Language = l.Name
	}
	return m
}

func (w *Workspace) backupDoc(path string) {
	e := w.docs[path]
	if e == nil || !backupable(e.doc) || !e.doc.Modified() || !w.hotExitOn() {
		return
	}
	d := e.doc
	snap := d.Buffer.Snapshot()
	v := snap.Version()
	if b := w.backupState(path); b.stored && b.written == v {
		return
	}
	w.hot.seq++
	seq, store, meta := w.hot.seq, w.hot.store, w.backupMeta(d)
	go func() {
		err := store.Write(seq, meta, snap, int64(snap.Len()))
		w.update(func() { w.backupWritten(path, v, err) })
	}()
}

func (w *Workspace) backupWritten(path string, version uint64, err error) {
	b := w.hot.docs[path]
	if b == nil || b.removed {
		return
	}
	switch {
	case errors.Is(err, backup.ErrTooLarge):
		b.tooLarge = true
	case err != nil:
		log.Printf("hot exit: %v", err)
	default:
		b.written, b.stored, b.tooLarge = version, true, false
	}
}

func (w *Workspace) forgetBackup(path string) {
	if b := w.hot.docs[path]; b != nil && b.timer != nil {
		b.timer.Stop()
	}
	w.hot.docs[path] = &backupDoc{removed: true}
}

func (w *Workspace) deleteBackup(path string) {
	w.forgetBackup(path)
	if !w.hasBackup(path) {
		return
	}
	w.hot.seq++
	seq, store := w.hot.seq, w.hot.store
	remove := func() {
		if err := store.Delete(seq, path); err != nil {
			log.Printf("hot exit: %v", err)
		}
	}
	if w.win == nil {
		remove()
		return
	}
	go remove()
}

func (w *Workspace) clearBackups() {
	w.stopBackupTimers()
	clear(w.hot.docs)
	clear(w.hot.pending)
	if w.hot.store == nil {
		return
	}
	w.hot.seq++
	seq, store := w.hot.seq, w.hot.store
	go func() {
		if err := store.Clear(seq); err != nil {
			log.Printf("hot exit: %v", err)
		}
	}()
}

func (w *Workspace) stopBackupTimers() {
	for _, b := range w.hot.docs {
		if b.timer != nil {
			b.timer.Stop()
		}
	}
}

func (w *Workspace) hotExitSettingsChanged() {
	if w.hot.store != nil && !w.hotExitOn() {
		w.clearBackups()
	}
}

func (w *Workspace) flushBackups() {
	if !w.hotExitOn() {
		return
	}
	docs := slices.DeleteFunc(w.modified(), func(d *document.Document) bool { return !backupable(d) })
	slices.SortStableFunc(docs, func(a, b *document.Document) int { return a.Buffer.Len() - b.Buffer.Len() })
	for _, d := range docs {
		b := w.backupState(d.Path)
		if b.timer != nil {
			b.timer.Stop()
		}
		snap := d.Buffer.Snapshot()
		v := snap.Version()
		if b.stored && b.written == v {
			continue
		}
		w.hot.seq++
		w.backupWritten(d.Path, v, w.hot.store.Write(w.hot.seq, w.backupMeta(d), snap, int64(snap.Len())))
	}
}

func (w *Workspace) backedUp(d *document.Document) bool {
	if !w.hotExitOn() || !backupable(d) {
		return false
	}
	b := w.hot.docs[d.Path]
	return b != nil && b.stored && !b.tooLarge && b.written == d.Buffer.Version()
}

func (w *Workspace) unbacked() []*document.Document {
	return slices.DeleteFunc(w.modified(), w.backedUp)
}

func (w *Workspace) PrepareClose() bool {
	w.flushBackups()
	return len(w.unbacked()) > 0
}

func (w *Workspace) takeBackup(path string) (backup.Meta, bool) {
	m, ok := w.hot.pending[path]
	delete(w.hot.pending, path)
	return m, ok && w.hot.store != nil
}

func (w *Workspace) restoreBackup(e *docEntry, path, text string, meta backup.Meta, err error) {
	name := w.tabTitle(path)
	if err != nil {
		log.Printf("hot exit: %v", err)
		w.setNotice("Cannot restore the unsaved changes to " + name + ".")
		return
	}
	if errors.Is(e.err, fs.ErrNotExist) {
		e.doc, e.err, e.kind = document.Empty(path), nil, viewer.None
	}
	d := e.doc
	if !backupable(d) {
		w.setNotice("Cannot restore the unsaved changes to " + name + ".")
		return
	}
	d.Buffer.Set(text)
	if meta.Language != "" && isUntitled(path) {
		d.Buffer.SetLanguage(syntax.Named(meta.Language))
	}
	b := w.backupState(path)
	b.written, b.stored = d.Buffer.Version(), true
	size, mod := d.DiskStamp()
	if onDisk(path) && (size != meta.DiskSize || !mod.Equal(meta.DiskModTime)) {
		w.setNotice("Restored the unsaved changes to " + name + ". The file changed on disk since then.")
	}
}

func (w *Workspace) dropPendingBackups(open map[string]bool) {
	for p := range w.hot.pending {
		if !open[p] {
			delete(w.hot.pending, p)
			w.deleteBackup(p)
		}
	}
	for p, b := range w.hot.docs {
		if b.removed && !open[p] {
			delete(w.hot.docs, p)
		}
	}
}

func (w *Workspace) discardBackups(docs []*document.Document) {
	if w.hot.store == nil {
		return
	}
	for _, d := range docs {
		w.forgetBackup(d.Path)
		w.hot.seq++
		if err := w.hot.store.Delete(w.hot.seq, d.Path); err != nil {
			log.Printf("hot exit: %v", err)
		}
	}
}

func (w *Workspace) loadBackups() {
	for p := range w.hot.pending {
		w.entry(p)
	}
}
