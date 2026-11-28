package prefs

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo/ui"
)

const (
	narrowViewWidth = 470
	wideViewWidth   = 900
	searchFontQuery = "font"
)

func newSettingsBed(t *testing.T, width int) (*SettingsView, *ui.Tester) {
	t.Helper()
	v := &SettingsView{Store: settings.Load(filepath.Join(t.TempDir(), "settings.json")), OpenJSON: func() {}}
	tt := ui.NewTester(v.View, width, testViewHeight)
	tt.Frame()
	tt.Frame()
	return v, tt
}

func TestSettingsHideCategoryListWhenNarrowAndOfferAPicker(t *testing.T) {
	t.Parallel()
	_, tt := newSettingsBed(t, narrowViewWidth)
	if _, ok := tt.Access(categoriesLabel); !ok {
		t.Fatalf("no category picker in %v", tt.Texts())
	}
	if _, ok := tt.AccessByRole(ui.RoleList, categoriesLabel); ok {
		t.Fatal("the category list takes width in a narrow view")
	}
	if b, ok := tt.AccessByRole(ui.RoleButton, settingsJSONOpen); !ok || b.Bounds.W > token.IconButtonSize.Px() {
		t.Fatalf("the JSON button is not an icon button: %+v", b.Bounds)
	}
}

func TestSettingsKeepCategoryListWhenWide(t *testing.T) {
	t.Parallel()
	_, tt := newSettingsBed(t, wideViewWidth)
	if _, ok := tt.AccessByRole(ui.RoleList, categoriesLabel); !ok {
		t.Fatal("no category list in a wide view")
	}
}

func TestSettingsSearchKeepsCategoriesWithMatches(t *testing.T) {
	t.Parallel()
	v, tt := newSettingsBed(t, wideViewWidth)
	v.query = searchFontQuery
	tt.Frame()
	tt.Frame()
	if _, ok := tt.AccessByRole(ui.RoleList, categoriesLabel); !ok {
		t.Fatal("the category list is gone while searching")
	}
	entries := v.navEntries(v.filter(), v.visible(v.filter()))
	if len(entries) == 0 || len(entries) >= len(settings.Categories) {
		t.Fatalf("entries = %+v, want only the categories with matches", entries)
	}
	if !tt.HasText(fmt.Sprintf("%s (%d)", entries[0].name, entries[0].count)) {
		t.Fatalf("no count for %q in %v", entries[0].name, tt.Texts())
	}
}

func TestTextSettingShowsItsDefaultAsPlaceholder(t *testing.T) {
	t.Parallel()
	for _, d := range settings.Defs {
		if d.Key == settings.FontFamily {
			if got := defaultHint(&d); got != token.FontEditor {
				t.Fatalf("font family hint = %q, want %q", got, token.FontEditor)
			}
		}
	}
}

func TestParseMessagesAreShortAndInline(t *testing.T) {
	t.Parallel()
	size := settings.DefFor(settings.FontSize)
	tests := []struct {
		def  *settings.Def
		in   string
		want string
	}{
		{size, "abc", "Enter a whole number."},
		{size, "12.5", "Enter a whole number."},
		{size, "3", "Enter a value from 6 to 72."},
		{size, "100", "Enter a value from 6 to 72."},
		{size, " 14 ", ""},
		{settings.DefFor(settings.LineHeight), "x", "Enter a number."},
		{settings.DefFor(settings.LineHeight), "9", "Enter a value from 1 to 3."},
		{settings.DefFor(settings.Rulers), "80, x", "Enter whole numbers separated by commas."},
		{settings.DefFor(settings.Rulers), "80, 120", ""},
	}
	for _, tt := range tests {
		_, err := parse(tt.def, tt.in)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != tt.want {
			t.Errorf("parse(%s, %q) = %q, want %q", tt.def.Key, tt.in, got, tt.want)
		}
	}
}

func controlFor(tt *ui.Tester, title string) (ui.AccessInfo, bool) {
	return tt.AccessByRole(ui.RoleTextField, title)
}

func TestInvalidInputStaysInlineAndKeepsTheDraft(t *testing.T) {
	t.Parallel()
	v, tt := newSettingsBed(t, wideViewWidth)
	in, ok := controlFor(tt, "Font Size")
	if !ok {
		t.Fatalf("no Font Size input in %v", tt.Texts())
	}
	tt.ClickAt(in.Bounds.X+in.Bounds.W/2, in.Bounds.Y+in.Bounds.H/2)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("abc")
	tt.Frame()
	tt.Frame()
	if !tt.HasText("Enter a whole number.") {
		t.Fatalf("no inline error while typing: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("Enter a whole number.") || v.drafts[settings.FontSize] != "abc" {
		t.Fatalf("the draft was dropped: %q %v", v.drafts[settings.FontSize], tt.Texts())
	}
	if v.err != "" || v.Store.Int(settings.FontSize) != 16 {
		t.Fatalf("err %q, font size %d", v.err, v.Store.Int(settings.FontSize))
	}
	tt.Key(0, ui.KeyBackspace)
	tt.Key(0, ui.KeyBackspace)
	tt.Key(0, ui.KeyBackspace)
	tt.Type("20")
	tt.Frame()
	tt.Frame()
	if tt.HasText("Enter a whole number.") {
		t.Fatalf("the error stays for a valid value: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if v.Store.Int(settings.FontSize) != 20 {
		t.Fatalf("font size = %d, want 20", v.Store.Int(settings.FontSize))
	}
}

func TestEscapeClearsTheInlineError(t *testing.T) {
	t.Parallel()
	v, tt := newSettingsBed(t, wideViewWidth)
	in, _ := controlFor(tt, "Font Size")
	tt.ClickAt(in.Bounds.X+in.Bounds.W/2, in.Bounds.Y+in.Bounds.H/2)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("abc")
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	if tt.HasText("Enter a whole number.") || v.drafts[settings.FontSize] == "abc" {
		t.Fatalf("Escape keeps the error: %v %v", v.drafts, tt.Texts())
	}
}

func TestBrokenFileShowsOneBannerAndDisablesControls(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{\n  \"editor.fontSize\": 14,\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opened := false
	v := &SettingsView{Store: settings.Load(path), OpenJSON: func() { opened = true }}
	tt := ui.NewTester(v.View, wideViewWidth, testViewHeight)
	tt.Frame()
	tt.Frame()
	banner := "settings.json has errors. Fix the file to change settings here."
	if _, ok := tt.AccessByRole(ui.RoleStatus, banner); !ok {
		t.Fatalf("no banner in %v", tt.Texts())
	}
	if tt.HasText("Unexpected end of file.") {
		t.Fatalf("a second banner repeats the error: %v", tt.Texts())
	}
	in, ok := controlFor(tt, "Font Size")
	if !ok || !in.Disabled {
		t.Fatalf("Font Size control %+v %v is not disabled", in, ok)
	}
	buttons := 0
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleButton && a.Name == settingsJSONOpen {
			buttons++
		}
	}
	if buttons != 2 {
		t.Fatalf("%d Open Settings (JSON) buttons, want toolbar plus banner", buttons)
	}
	if err := tt.Click(settingsJSONOpen); err != nil {
		t.Fatal(err)
	}
	if !opened {
		t.Fatal("the banner action does not open the file")
	}
}

func TestSelectShowsReadableLabels(t *testing.T) {
	t.Parallel()
	_, tt := newSettingsBed(t, wideViewWidth)
	if tt.HasText("wordWrapColumn") || tt.HasText("onExitAndWindowClose") {
		t.Fatalf("raw option values show: %v", tt.Texts())
	}
}
