package workspace

type ViewState struct {
	Project    bool
	Editor     bool
	TextEditor bool
	OnDisk     bool
	Git        bool
	Sidebar    bool
	Panel      bool
	Back       bool
	Forward    bool
	WordWrap   bool
}

func (w *Workspace) ViewState() ViewState {
	path, diff := w.wrapTarget()
	wrap, _ := w.wrapping(diff, path)
	s := ViewState{
		Project:  true,
		Git:      w.git != nil && w.git.Ready(),
		Sidebar:  !w.sidebarHidden,
		Panel:    w.tools.panelOpen,
		Back:     w.canNavigate(true),
		Forward:  w.canNavigate(false),
		WordWrap: wrap,
	}
	if t := w.active.ActiveTab(); t != nil {
		s.Editor, s.TextEditor, s.OnDisk = true, t.Editor != nil, onDisk(t.Path)
	}
	return s
}

func (w *Workspace) publishViewState() {
	state := w.ViewState()
	if state == w.viewState {
		return
	}
	w.viewState = state
	if w.OnViewState != nil {
		w.OnViewState(state)
	}
}
