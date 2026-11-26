package editorconfig

import (
	"io/fs"
	"path/filepath"
	"testing"
)

func TestParse(t *testing.T) {
	f, err := Parse([]byte("\ufeff# top\nroot = true\n\n[*]\nindent_style = Space\n; note\nindent_size=2\n[*.go]\nindent_style = tab\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !f.Root || len(f.Sections) != 2 {
		t.Fatalf("file = %+v", f)
	}
	if f.Sections[0].Glob != "*" || f.Sections[0].Props["indent_style"] != "space" || f.Sections[0].Props["indent_size"] != "2" {
		t.Fatalf("section 0 = %+v", f.Sections[0])
	}
}

func TestParseReportsInvalidLines(t *testing.T) {
	f, err := Parse([]byte("[*]\nnot a pair\nindent_size = 4\n"))
	if err == nil {
		t.Fatal("no error for an invalid line")
	}
	if f.Sections[0].Props["indent_size"] != "4" {
		t.Fatalf("valid lines after the error were dropped: %+v", f)
	}
}

func TestResolve(t *testing.T) {
	root := filepath.FromSlash("/work")
	files := map[string]string{
		"/.editorconfig":                "[*]\nindent_size = 8\n",
		"/work/.editorconfig":           "root = true\n[*]\nindent_style = space\nindent_size = 4\nend_of_line = lf\n[*.go]\nindent_style = tab\nindent_size = unset\n",
		"/work/web/.editorconfig":       "[*.{js,ts}]\nindent_size = 2\ninsert_final_newline = true\n[lib/**]\ncharset = latin1\n",
		"/work/web/lib/x/.editorconfig": "",
	}
	read := func(name string) ([]byte, error) {
		if s, ok := files[filepath.ToSlash(name)]; ok {
			return []byte(s), nil
		}
		return nil, fs.ErrNotExist
	}
	props, consulted, err := Resolve(filepath.Join(root, "web", "lib", "x", "app.ts"), read)
	if err != nil {
		t.Fatal(err)
	}
	want := Settings{IndentStyle: IndentSpaces, IndentSize: 2, TabWidth: 2, EndOfLine: "\n", Charset: "latin1", InsertFinalNewline: On}
	if got := props.Settings(); got != want {
		t.Fatalf("settings = %+v, want %+v", got, want)
	}
	if len(consulted) != 4 {
		t.Fatalf("consulted = %v, want 4 files up to the root one", consulted)
	}
	props, _, err = Resolve(filepath.Join(root, "main.go"), read)
	if err != nil {
		t.Fatal(err)
	}
	if got := props.Settings(); got.IndentStyle != IndentTabs || got.IndentSize != 0 || got.EndOfLine != "\n" {
		t.Fatalf("go settings = %+v", got)
	}
}

func TestSettingsIndentSizeTab(t *testing.T) {
	s := Props{"indent_style": "tab", "indent_size": "tab", "tab_width": "3"}.Settings()
	if s.IndentSize != 3 || s.TabWidth != 3 {
		t.Fatalf("settings = %+v", s)
	}
	s = Props{"indent_size": "5"}.Settings()
	if s.TabWidth != 5 {
		t.Fatalf("tab width = %d, want the indent size", s.TabWidth)
	}
	s = Props{"trim_trailing_whitespace": "false", "max_line_length": "off"}.Settings()
	if s.TrimTrailingWhitespace != Off || s.MaxLineLength != 0 {
		t.Fatalf("settings = %+v", s)
	}
	if !(Props{}).Settings().IsZero() {
		t.Fatal("empty props give settings")
	}
}
