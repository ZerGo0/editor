package settings

import (
	"os"
	"path/filepath"
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
