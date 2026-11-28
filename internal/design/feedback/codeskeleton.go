package feedback

import (
	"strings"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	SkeletonCodeLines            = 40
	skeletonDiffAdds             = 3
	skeletonDiffDels             = 2
	skeletonDiffMaxLines         = 60
	skeletonDiffAround           = 8
	skeletonFontSize     float32 = 13
	codeIndent                   = "    "
	codeWordsMin                 = 2
	codeWordsSpan                = 4
	lineEnd                      = "\n"
)

const (
	SkeletonBase = iota
	SkeletonOld
	SkeletonNew
)

var (
	codeIndents = []int{0, 1, 1, 2, 2, 1, 0, 0, 1, 2, 3, 2, 1, 0}
	versionInks = []string{placeholderInk, "o", "z"}
)

type codeShape struct{ lines int }

type diffShape struct{ adds, dels int }

func SkeletonCode(first, lines, version int) string {
	ink := versionInks[version%len(versionInks)]
	var b strings.Builder
	for i := first; i < first+lines; i++ {
		b.WriteString(strings.Repeat(codeIndent, codeIndents[i%len(codeIndents)]))
		b.WriteString(placeholder(i, codeWordsMin+i%codeWordsSpan, ink))
		b.WriteString(lineEnd)
	}
	return b.String()
}

func SkeletonDiff(adds, dels int) (oldText, newText string) {
	if adds < 0 || dels < 0 {
		adds, dels = skeletonDiffAdds, skeletonDiffDels
	}
	adds, dels = min(adds, skeletonDiffMaxLines), min(dels, skeletonDiffMaxLines)
	lead := SkeletonCode(0, skeletonDiffAround, SkeletonBase)
	tail := SkeletonCode(skeletonDiffAround+max(adds, dels), skeletonDiffAround, SkeletonBase)
	return lead + SkeletonCode(skeletonDiffAround, dels, SkeletonOld) + tail, lead + SkeletonCode(skeletonDiffAround, adds, SkeletonNew) + tail
}

func SkeletonEditor(c *ui.Context, st *editor.State) {
	if p := token.Of(c); p.IsSkeleton() {
		st.Font, st.Theme, st.NoActiveLine = skeletonFont(st.Font), p.CodeTheme(), true
	}
}

func skeletonFont(f ui.Font) ui.Font {
	f.Family = token.FontSkeleton
	if f.Size == 0 {
		f.Size = skeletonFontSize
	}
	return f
}

func CodeSkeleton(c *ui.Context, label string, lines int, configure func(*editor.State)) ui.Element {
	return Skeleton(c, label, func() {
		host := ui.Column(c).Grow(1).FillWidth().MinHeight(0)
		st := *ui.Local(host, codeShape{lines}, func() *editor.State {
			text := strings.TrimSuffix(SkeletonCode(0, lines, SkeletonBase), lineEnd)
			return &editor.State{Buffer: editor.NewBuffer(text), ReadOnly: true}
		})
		host.Children(func() {
			if configure != nil {
				configure(st)
			}
			SkeletonEditor(c, st)
			editor.View(c, st).Grow(1).FillWidth().MinHeight(0)
		})
	}).Grow(1)
}

func DiffSkeleton(c *ui.Context, label string, adds, dels int, configure func(*editor.DiffState)) ui.Element {
	return Skeleton(c, label, func() {
		host := ui.Column(c).Grow(1).FillWidth().MinHeight(0)
		d := *ui.Local(host, diffShape{adds, dels}, func() *editor.DiffState {
			oldText, newText := SkeletonDiff(adds, dels)
			return &editor.DiffState{Old: editor.NewBuffer(oldText), New: editor.NewBuffer(newText)}
		})
		host.Children(func() {
			if configure != nil {
				configure(d)
			}
			d.Font, d.Theme, d.Editable = skeletonFont(d.Font), token.Of(c).CodeTheme(), false
			d.LineSelection, d.OnGutterButton = false, nil
			editor.DiffView(c, d).Grow(1).FillWidth().MinHeight(0)
		})
	}).Grow(1)
}
