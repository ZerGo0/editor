package diffmode

import (
	"context"
	"fmt"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui/editor"
)

func stampOf(d *editor.DiffState) stamp {
	return stamp{old: d.Old, new: d.New, oldVer: d.Old.Version(), newVer: d.New.Version()}
}

func (e *Engine) cancelMoves() {
	e.moveGen++
	if e.moveCancel != nil {
		e.moveCancel()
		e.moveCancel = nil
	}
	if e.moveTimer != nil {
		e.moveTimer.Stop()
		e.moveTimer = nil
	}
}

func (e *Engine) clearMoves() {
	e.cancelMoves()
	e.moves, e.annots, e.stamp = nil, nil, stamp{}
}

func (e *Engine) syncMoves() {
	d := e.State
	if !Session.ShowMoves {
		if e.stamp != (stamp{}) {
			e.clearMoves()
		}
		return
	}
	if e.busy || d.Old == nil || d.New == nil || d.Comparing() {
		return
	}
	st := stampOf(d)
	if st == e.stamp {
		return
	}
	edited := st.old == e.stamp.old && st.new == e.stamp.new
	e.cancelMoves()
	e.moves, e.annots, e.stamp = nil, nil, st
	if !edited {
		e.runMoves()
		return
	}
	gen := e.moveGen
	e.moveTimer = e.after(e.Delay, func() {
		if gen == e.moveGen && stampOf(d) == st {
			e.runMoves()
		}
	})
}

func (e *Engine) runMoves() {
	d := e.State
	added, deleted := d.Stats()
	if added+deleted > MaxMoveChangedLines {
		return
	}
	hunks := d.Hunks()
	if len(hunks) == 0 {
		return
	}
	oldText, newText := d.Old.String(), d.New.String()
	ctx, cancel := context.WithCancel(e.ctx)
	e.moveCancel = cancel
	gen := e.moveGen
	applog.Go(func() {
		moves := DetectMoves(ctx, hunks, oldText, newText)
		e.Post(func() {
			if gen == e.moveGen && ctx.Err() == nil {
				e.setMoves(moves)
			}
		})
	})
}

func (e *Engine) setMoves(moves []Move) {
	e.moves = moves
	e.annots = make([]editor.DiffAnnotation, 0, 2*len(moves))
	for i, m := range moves {
		e.annots = append(e.annots,
			editor.DiffAnnotation{Side: editor.DiffOld, Line: m.OldStart, Height: token.ControlHeightSmall.Px(), Key: moveKey{i, editor.DiffOld}},
			editor.DiffAnnotation{Side: editor.DiffNew, Line: m.NewStart, Height: token.ControlHeightSmall.Px(), Key: moveKey{i, editor.DiffNew}},
		)
	}
}

func (e *Engine) lineNumber(side editor.DiffSide, line int) int {
	if e.LineNumber != nil {
		return e.LineNumber(side, line)
	}
	return line + 1
}

func (e *Engine) moveLabel(m Move, from editor.DiffSide) string {
	if from == editor.DiffOld {
		return fmt.Sprintf("Moved to line %d", e.lineNumber(editor.DiffNew, m.NewStart))
	}
	return fmt.Sprintf("Moved from line %d", e.lineNumber(editor.DiffOld, m.OldStart))
}
