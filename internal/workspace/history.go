package workspace

import (
	"github.com/ZerGo0/editor/internal/history"
)

const externalChangeLabel = "External change"

func (w *Workspace) historyUI() *history.UI {
	if w.history == nil && w.env.History != nil {
		w.history = history.NewUI(w, w.env.History)
	}
	return w.history
}

func (w *Workspace) recordHistory(path, label string) {
	h, d := w.historyUI(), w.doc(path)
	if h == nil || d == nil || d.Buffer == nil || int64(d.Buffer.Len()) > history.DefaultMaxFileBytes {
		return
	}
	h.Record(path, []byte(d.Buffer.String()), label)
}

func (w *Workspace) RunHistory(id string) {
	if h := w.historyUI(); h != nil {
		h.Run(id)
	}
}

func (w *Workspace) historyRenamed(from, to string) {
	if h := w.historyUI(); h != nil {
		h.Renamed(from, to)
	}
}
