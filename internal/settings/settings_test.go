package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONSetting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	data := `{"` + IssueQueries + `": [{"label": "Mine", "query": "is:open"}]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Load(path)
	if len(s.Errors) != 0 {
		t.Fatalf("errors: %v", s.Errors)
	}
	if got := string(s.JSON(IssueQueries)); got != `[{"label": "Mine", "query": "is:open"}]` {
		t.Fatalf("JSON = %s", got)
	}
	if s.JSON(GitAutoFetch) != nil {
		t.Fatal("JSON of a non-JSON setting")
	}
}

func TestJSONSettingDefaultIsEmpty(t *testing.T) {
	s := Load(filepath.Join(t.TempDir(), "missing.json"))
	if len(s.JSON(IssueQueries)) != 0 {
		t.Fatalf("default = %s", s.JSON(IssueQueries))
	}
}

func TestFolderSettingsOverrideUserSettings(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "settings.json")
	folderPath := filepath.Join(dir, "project", ".editor", "settings.json")
	if err := os.WriteFile(userPath, []byte(`{"editor.tabSize": 8, "editor.folding": false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	user := Load(userPath)
	folder := LoadFolder(folderPath, user)
	if folder.Int(TabSize) != 8 || folder.Bool(Folding) || folder.IsSet(TabSize) {
		t.Fatalf("folder without a file: tabSize %d folding %v set %v", folder.Int(TabSize), folder.Bool(Folding), folder.IsSet(TabSize))
	}
	if err := folder.Set(TabSize, 2); err != nil {
		t.Fatal(err)
	}
	if folder.Int(TabSize) != 2 || user.Int(TabSize) != 8 || !folder.IsSet(TabSize) {
		t.Fatalf("after Set: folder %d user %d", folder.Int(TabSize), user.Int(TabSize))
	}
	if folder.Int(FontSize) != defaultFontSize {
		t.Fatalf("default font size = %d", folder.Int(FontSize))
	}
	if err := folder.Reset(TabSize); err != nil {
		t.Fatal(err)
	}
	if folder.Int(TabSize) != 8 {
		t.Fatalf("after Reset: %d", folder.Int(TabSize))
	}
}

func TestFolderSettingsRejectAppWideKeys(t *testing.T) {
	dir := t.TempDir()
	folderPath := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(folderPath, []byte(`{"window.zoomLevel": 3, "editor.tabSize": 3}`), 0o644); err != nil {
		t.Fatal(err)
	}
	folder := LoadFolder(folderPath, Load(filepath.Join(dir, "user.json")))
	if folder.Int(ZoomLevel) != 0 || folder.Int(TabSize) != 3 {
		t.Fatalf("zoom %d tabSize %d", folder.Int(ZoomLevel), folder.Int(TabSize))
	}
	if len(folder.Errors) != 1 {
		t.Fatalf("errors = %v", folder.Errors)
	}
	if err := folder.Set(ZoomLevel, 1); err == nil {
		t.Fatal("Set of an app-wide key in folder settings succeeded")
	}
}

func loadText(t *testing.T, text string) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return Load(path)
}

func TestErrorsArePlainAndLocated(t *testing.T) {
	t.Parallel()
	s := loadText(t, "{\n  // note\n  \"editor.tabSize\": \"wide\",\n  \"editor.nope\": 1,\n  \"editor.fontSize\": 14.5\n}")
	want := []struct {
		text      string
		line, col int
	}{
		{"editor.fontSize: must be a whole number.", 4, 2},
		{"editor.nope: unknown setting.", 3, 2},
		{"editor.tabSize: expected a number, but found a string.", 2, 2},
	}
	if s.Broken || len(s.Errors) != len(want) {
		t.Fatalf("broken %v, errors %+v", s.Broken, s.Errors)
	}
	for i, w := range want {
		e := s.Errors[i]
		if e.Text != w.text || e.Line != w.line || e.Col != w.col {
			t.Errorf("error %d = %+v, want %+v", i, e, w)
		}
		if strings.Contains(e.Text, "json:") || strings.Contains(e.Text, "Go ") {
			t.Errorf("error %d has Go jargon: %q", i, e.Text)
		}
	}
}

func TestSyntaxErrorBreaksTheStore(t *testing.T) {
	t.Parallel()
	s := loadText(t, "{\n  \"editor.fontSize\": 14,\n  \"editor.tabSize\" 2\n}")
	if !s.Broken || len(s.Errors) != 1 {
		t.Fatalf("broken %v, errors %+v", s.Broken, s.Errors)
	}
	if e := s.Errors[0]; e.Text != "Unexpected character '2'." || e.Line != 2 || e.Col != 19 {
		t.Fatalf("error = %+v", e)
	}
	if s.Int(FontSize) != defaultFontSize {
		t.Fatal("a broken file changes a value")
	}
}

func TestTruncatedFileHasAPosition(t *testing.T) {
	t.Parallel()
	s := loadText(t, "{\n  \"editor.fontSize\": 14")
	if e := s.Errors[0]; !s.Broken || e.Text != "Unexpected end of file." || e.Line != 1 {
		t.Fatalf("errors = %+v", s.Errors)
	}
}

func TestOptionLabels(t *testing.T) {
	t.Parallel()
	for _, d := range Defs {
		if d.Kind != Enum {
			continue
		}
		for _, o := range d.Options {
			if label := d.OptionLabel(o); label == "" {
				t.Errorf("%s: empty label for %q", d.Key, o)
			}
		}
	}
	if got := DefFor(WordWrap).OptionLabel(WrapColumn); got != "Wrap at Column" {
		t.Fatalf("label = %q", got)
	}
	if got := DefFor(GitPullMode).OptionLabel(PullFFOnly); got != "Fast-Forward Only" {
		t.Fatalf("label = %q", got)
	}
}

func TestTitlesAreUnique(t *testing.T) {
	t.Parallel()
	seen := map[string]string{}
	for _, d := range Defs {
		if other, ok := seen[d.Title]; ok {
			t.Errorf("%s and %s share the title %q", d.Key, other, d.Title)
		}
		seen[d.Title] = d.Key
	}
}
