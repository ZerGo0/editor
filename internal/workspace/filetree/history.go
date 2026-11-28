package filetree

const undoLimit = 50

type Move struct {
	From, To string
}

type Trashed struct {
	Path, Trash string
}

type Change struct {
	Label   string
	Moves   []Move
	Created []Trashed
	Removed []Trashed
}

type history struct {
	undo, redo []*Change
	busy       bool
}

func (t *Tree) Record(ch *Change) {
	t.PushUndo(ch)
	t.history.redo = nil
}

func (t *Tree) PushUndo(ch *Change) {
	h := &t.history
	h.undo = append(h.undo, ch)
	if len(h.undo) > undoLimit {
		h.undo = h.undo[len(h.undo)-undoLimit:]
	}
}

func (t *Tree) PushRedo(ch *Change) {
	t.history.redo = append(t.history.redo, ch)
}

func (t *Tree) PopUndo() *Change {
	return t.history.pop(&t.history.undo)
}

func (t *Tree) PopRedo() *Change {
	return t.history.pop(&t.history.redo)
}

func (t *Tree) Settled() {
	t.history.busy = false
}

func (h *history) pop(stack *[]*Change) *Change {
	s := *stack
	if h.busy || len(s) == 0 {
		return nil
	}
	ch := s[len(s)-1]
	*stack = s[:len(s)-1]
	h.busy = true
	return ch
}
