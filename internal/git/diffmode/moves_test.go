package diffmode

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

func numbered(prefix string, from, to int) []string {
	var lines []string
	for i := from; i < to; i++ {
		lines = append(lines, fmt.Sprintf("%s statement number %d()", prefix, i))
	}
	return lines
}

func joinLines(parts ...[]string) string {
	var all []string
	for _, p := range parts {
		all = append(all, p...)
	}
	return strings.Join(all, "\n") + "\n"
}

func detect(oldText, newText string) []Move {
	d := &editor.DiffState{Old: editor.NewBuffer(oldText), New: editor.NewBuffer(newText)}
	return DetectMoves(context.Background(), d.Hunks(), oldText, newText)
}

func TestDetectMovesFindsMovedBlock(t *testing.T) {
	head, block, mid, tail := numbered("head", 0, 6), numbered("moved", 0, 4), numbered("mid", 0, 8), numbered("tail", 0, 6)
	oldText := joinLines(head, block, mid, tail)
	newText := joinLines(head, mid, tail, block)
	got := detect(oldText, newText)
	want := Move{OldStart: 6, OldEnd: 10, NewStart: 20, NewEnd: 24}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("moves = %+v, want [%+v]", got, want)
	}
}

func TestDetectMovesRequiresEnoughText(t *testing.T) {
	head, tail := numbered("head", 0, 6), numbered("tail", 0, 20)
	short := []string{"a := 1", "b := 2"}
	if got := detect(joinLines(head, short, tail), joinLines(head, tail, short)); len(got) != 0 {
		t.Fatalf("two short lines moved: %+v", got)
	}
	long := []string{strings.Repeat("x", MinMoveChars/2+1), strings.Repeat("y", MinMoveChars/2+1)}
	if got := detect(joinLines(head, long, tail), joinLines(head, tail, long)); len(got) != 1 {
		t.Fatalf("two long lines not moved: %+v", got)
	}
}

func TestDetectMovesIgnoresBlockVariations(t *testing.T) {
	indent := func(block []string) []string {
		out := make([]string, len(block))
		for i, l := range block {
			out[i] = "\t\t" + l + "  "
		}
		return out
	}
	blankEdges := func(block []string) []string { return append(append([]string{""}, block...), "") }
	same := func(block []string) []string { return block }
	for _, tc := range []struct {
		name          string
		before, after func([]string) []string
	}{
		{"indentation", same, indent},
		{"blank edges", blankEdges, blankEdges},
	} {
		head, block, tail := numbered("head", 0, 6), numbered("moved", 0, 3), numbered("tail", 0, 20)
		got := detect(joinLines(head, tc.before(block), tail), joinLines(head, tail, tc.after(block)))
		if len(got) != 1 || got[0].OldEnd-got[0].OldStart != 3 {
			t.Errorf("%s: moves = %+v", tc.name, got)
		}
	}
}

func TestDetectMovesIgnoresReindentInPlace(t *testing.T) {
	head, block, tail := numbered("head", 0, 6), numbered("body", 0, 5), numbered("tail", 0, 6)
	indented := make([]string, len(block))
	for i, l := range block {
		indented[i] = "    " + l
	}
	if got := detect(joinLines(head, block, tail), joinLines(head, indented, tail)); len(got) != 0 {
		t.Fatalf("reindent in place reported as a move: %+v", got)
	}
}

func TestDetectMovesUsesEachLineOnce(t *testing.T) {
	head, block, tail := numbered("head", 0, 6), numbered("moved", 0, 4), numbered("tail", 0, 20)
	got := detect(joinLines(head, block, tail), joinLines(head, tail, block, numbered("gap", 0, 5), block))
	if len(got) != 1 {
		t.Fatalf("moves = %+v", got)
	}
}

func TestDetectMovesPrefersLongestMatch(t *testing.T) {
	head, tail := numbered("head", 0, 6), numbered("tail", 0, 20)
	block := numbered("moved", 0, 6)
	oldText := joinLines(head, block, tail)
	newText := joinLines(block[:3], head, tail, block)
	got := detect(oldText, newText)
	if len(got) != 1 || got[0].OldEnd-got[0].OldStart != 6 {
		t.Fatalf("moves = %+v", got)
	}
}

func TestDetectMovesNoHunks(t *testing.T) {
	text := joinLines(numbered("same", 0, 10))
	if got := detect(text, text); len(got) != 0 {
		t.Fatalf("moves = %+v", got)
	}
}

func TestDetectMovesStopsWhenCancelled(t *testing.T) {
	head, block, mid := numbered("head", 0, 6), numbered("moved", 0, 4), numbered("mid", 0, 8)
	oldText, newText := joinLines(head, block, mid), joinLines(head, mid, block)
	d := &editor.DiffState{Old: editor.NewBuffer(oldText), New: editor.NewBuffer(newText)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := DetectMoves(ctx, d.Hunks(), oldText, newText); got != nil {
		t.Fatalf("moves = %+v", got)
	}
}

func TestDetectMovesToleratesBadRanges(t *testing.T) {
	hunks := []editor.DiffHunk{{OldStart: -3, OldEnd: 99, NewStart: 5, NewEnd: 1}}
	_ = DetectMoves(context.Background(), hunks, "a\nb\n", "b\na\n")
}
