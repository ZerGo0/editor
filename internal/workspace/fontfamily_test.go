package workspace

import (
	"slices"
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/settings"
)

func TestFontFamiliesParsesQuotedLists(t *testing.T) {
	for list, want := range map[string][]string{
		"'Hack', Hack":                    {"Hack", "Hack"},
		`"Fira Code" , 'Menlo',monospace`: {"Fira Code", "Menlo", "monospace"},
		" , ''":                           nil,
		"":                                nil,
	} {
		if got := fontFamilies(list); !slices.Equal(got, want) {
			t.Errorf("fontFamilies(%q) = %q, want %q", list, got, want)
		}
	}
}

func TestPickFontFamilyUsesFirstInstalled(t *testing.T) {
	installed := func() map[string]string { return map[string]string{"menlo": "Menlo", "hack": "Hack"} }
	for list, want := range map[string]string{
		"'Missing', menlo, Hack": "Menlo",
		"Missing, 'HACK'":        "Hack",
		"Missing":                token.FontEditor,
		"":                       token.FontEditor,
	} {
		if got := pickFontFamily(list, installed); got != want {
			t.Errorf("pickFontFamily(%q) = %q, want %q", list, got, want)
		}
	}
}

func TestDefaultEditorFontIsHack16(t *testing.T) {
	const (
		wantFamily = "Hack"
		wantSize   = 16
	)
	env := newTestEnv(t)
	w := newTestWorkspace(t, t.TempDir(), env, session.Project{})
	if f := w.editorFont(); f.Family != wantFamily || f.Size != wantSize {
		t.Errorf("default editor font = %q %v, want %q %d", f.Family, f.Size, wantFamily, wantSize)
	}
	for _, key := range []string{settings.ScmInputFontSize, settings.MarkdownFontSize} {
		if got := env.Settings.Int(key); got != wantSize {
			t.Errorf("default %s = %d, want %d", key, got, wantSize)
		}
	}
	if got := w.viewerOptions().PreviewSize; got != wantSize {
		t.Errorf("Markdown preview font size = %v, want %d", got, wantSize)
	}
}
