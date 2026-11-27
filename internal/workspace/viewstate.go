package workspace

type ViewState struct {
	WordWrap bool
}

func (w *Workspace) ViewState() ViewState {
	path, diff := w.wrapTarget()
	wrap, _ := w.wrapping(diff, path)
	return ViewState{WordWrap: wrap}
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
