package workspace

import (
	"path/filepath"
	"testing"

	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/ZerGo0/editor/internal/testutil"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const trojanSource = "var isAdmin = false\n/*\u202E } \u2066if (isAdmin)\u2069 \u2066 begin admins only */\nlet a\u200Bb = 1\nlet \u0441ount = f(x)\n"

func unicodeMarks(rs []editor.Replacement) (marks, brackets int) {
	for _, r := range rs {
		if isBracketColor(r) {
			brackets++
		} else {
			marks++
		}
	}
	return marks, brackets
}

func TestUnicodeMarksShowTrojanSourceNextToBracketColors(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{"trojan.js": trojanSource})
	env := newTestEnv(t)
	w := newTestWorkspace(t, root, env, session.Project{})
	tab, _ := openTab(t, w, filepath.Join(root, "trojan.js"))
	tt := decoratedTester(w, tab)
	marks, brackets := unicodeMarks(tab.Editor.Replacements())
	if marks != 6 || brackets == 0 {
		t.Fatalf("marks %d brackets %d, want 4 bidi controls, a zero-width space and a Cyrillic letter beside bracket colors", marks, brackets)
	}
	if _, ok := tt.AccessByRole(ui.RoleImage, "The character U+202E is a control character that can change how the text around it shows"); !ok {
		t.Fatal("the bidi control mark has no accessible name")
	}
	if err := env.Settings.Set(settings.ControlCharacters, false); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if marks, brackets := unicodeMarks(tab.Editor.Replacements()); marks != 6 || brackets == 0 {
		t.Fatalf("with control characters off, the bidi controls must show as invisible characters: marks %d brackets %d", marks, brackets)
	}
	for _, key := range []string{settings.UnicodeInvisible, settings.UnicodeAmbiguous} {
		if err := env.Settings.Set(key, false); err != nil {
			t.Fatal(err)
		}
	}
	tt.Frame()
	if marks, brackets := unicodeMarks(tab.Editor.Replacements()); marks != 0 || brackets == 0 {
		t.Fatalf("all highlights off: marks %d brackets %d", marks, brackets)
	}
}

func TestUnicodeMarksFollowEditsAndSkipProse(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{"a.go": "package a\n", "notes.md": "a\u200Bb \u0441at \u202E\n"})
	env := newTestEnv(t)
	w := newTestWorkspace(t, root, env, session.Project{})
	code, _ := openTab(t, w, filepath.Join(root, "a.go"))
	prose, _ := openTab(t, w, filepath.Join(root, "notes.md"))
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			editor.View(c.Key("code"), code.Editor).Grow(1)
			editor.View(c.Key("prose"), prose.Editor).Grow(1)
		})
		w.decorateCode(c, code)
		w.decorateCode(c, prose)
	}, testViewW, testViewH)
	tt.Frame()
	if marks, _ := unicodeMarks(code.Editor.Replacements()); marks != 0 {
		t.Fatalf("plain Go has %d marks", marks)
	}
	code.Editor.Insert("// \u202E")
	tt.Frame()
	if marks, _ := unicodeMarks(code.Editor.Replacements()); marks != 1 {
		t.Fatalf("a typed bidi control gives %d marks, want 1", marks)
	}
	if marks, _ := unicodeMarks(prose.Editor.Replacements()); marks != 1 {
		t.Fatalf("Markdown shows %d marks, want only the control character as VS Code does", marks)
	}
}
