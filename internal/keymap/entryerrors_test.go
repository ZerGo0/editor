package keymap_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/jsonc"
	"github.com/ZerGo0/editor/internal/keymap"
)

func loadKeys(t *testing.T, text string) *keymap.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "keybindings.json")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return keymap.Load(path)
}

func loadProblem(t *testing.T, text string) jsonc.Problem {
	t.Helper()
	errs := loadKeys(t, text).Current.Errors
	if len(errs) != 1 {
		t.Fatalf("errors = %+v", errs)
	}
	return errs[0]
}

func TestBadKeyTypeIsPlainAndLocated(t *testing.T) {
	t.Parallel()
	e := loadProblem(t, "[\n  {\"key\": \"cmd+k\", \"command\": \"x\"},\n  // second\n  {\"key\": 5, \"command\": \"y\"}\n]")
	if e.Text != `keybindings.json, entry 2: "key" must be a string.` || e.Line != 3 || e.Col != 2 {
		t.Fatalf("error = %+v", e)
	}
	if strings.Contains(e.Text, "json:") || strings.Contains(e.Text, "Go ") || strings.Contains(e.Text, string(filepath.Separator)) {
		t.Fatalf("error has jargon or a path: %q", e.Text)
	}
}

func TestSyntaxErrorInKeybindingsIsLocated(t *testing.T) {
	t.Parallel()
	if e := loadProblem(t, "[\n  {\"key\": \"cmd+k\" \"command\": \"x\"}\n]"); e.Text != "keybindings.json: Unexpected character '\"'." || e.Line != 1 || e.Col != 18 {
		t.Fatalf("error = %+v", e)
	}
}

func TestEntryErrorsKnowTheirLine(t *testing.T) {
	t.Parallel()
	if e := loadProblem(t, "[\n  {\"key\": \"cmd+k\", \"command\": \"x\"},\n  {\"key\": \"hyper+p\", \"command\": \"y\"}\n]"); !strings.HasPrefix(e.Text, "entry 2:") || e.Line != 2 || e.Col != 2 {
		t.Fatalf("error = %+v", e)
	}
}

func TestEntriesWithoutAFileHaveNoPosition(t *testing.T) {
	t.Parallel()
	k := keymap.New(nil, []keymap.Entry{{Key: "cmd+p"}})
	if len(k.Errors) != 1 || k.Errors[0].Located() {
		t.Fatalf("errors = %+v", k.Errors)
	}
}

func TestKeybindingsMustBeAList(t *testing.T) {
	t.Parallel()
	s := loadKeys(t, `{"key": "cmd+k"}`)
	if len(s.Current.Errors) != 1 || s.Current.Errors[0].Text != "keybindings.json: expected a list, but found an object." {
		t.Fatalf("errors = %+v", s.Current.Errors)
	}
}
