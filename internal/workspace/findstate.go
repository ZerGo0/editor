package workspace

import "github.com/ZerGo0/editor/internal/findwidget"

const findWidgetStateKey = "findwidget.state"

func (w *Workspace) initFindWidget() {
	sh := &findwidget.Shared{}
	w.views.Load(findWidgetStateKey, sh)
	w.views.Track(findWidgetStateKey, func() any { return sh })
	w.search.widget = sh
}
