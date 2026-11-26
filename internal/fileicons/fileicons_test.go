package fileicons

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestForLookupOrder(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"go.mod", "go-mod"},
		{"package.json", "nodejs"},
		{"data.json", "json"},
		{"Dockerfile", "docker"},
		{"DOCKERFILE", "docker"},
		{".gitignore", "git"},
		{"README.md", "readme"},
		{"notes.md", "markdown"},
		{"main.go", "go"},
		{"app.test.ts", "test-ts"},
		{"App.Spec.TS", "test-ts"},
		{"index.d.ts", "typescript-def"},
		{"index.ts", "typescript"},
		{"a.b.c.ts", "typescript"},
		{"src/pkg/go.mod", "go-mod"},
		{"image.PNG", "image"},
		{"Makefile", "makefile"},
		{".env", "tune"},
		{"unknown.zzzzz", "file"},
		{"noextension", "file"},
		{"", "file"},
		{".", "file"},
	}
	for _, tt := range tests {
		if got := For(tt.name).Name(); got != tt.want {
			t.Errorf("For(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestForFileLanguageFallback(t *testing.T) {
	if got := ForFile("script", "python").Name(); got != "python" {
		t.Errorf("language fallback = %q, want python", got)
	}
	if got := ForFile("main.go", "python").Name(); got != "go" {
		t.Errorf("file name wins over language = %q, want go", got)
	}
	if got := ForFile("script", "no-such-language").Name(); got != defaultFileIcon {
		t.Errorf("unknown language = %q, want %q", got, defaultFileIcon)
	}
	if got := ForLanguage("TypeScript").Name(); got != "typescript" {
		t.Errorf("ForLanguage is case-insensitive: got %q", got)
	}
	if got := ForLanguage("").Name(); got != defaultFileIcon {
		t.Errorf("ForLanguage(\"\") = %q, want %q", got, defaultFileIcon)
	}
}

func TestFolderNamesAndOpenState(t *testing.T) {
	tests := []struct {
		name       string
		wantClosed string
	}{
		{"src", "folder-src"},
		{"SRC", "folder-src"},
		{"node_modules", "folder-node"},
		{".github", "folder-github"},
		{"_src", "folder-src"},
		{"__tests__", "folder-test"},
		{"docs/", "folder-docs"},
		{"repo/internal/src", "folder-src"},
		{".github/workflows", "folder-gh-workflows"},
		{"no-such-folder", defaultFolderIcon},
		{"", defaultFolderIcon},
	}
	for _, tt := range tests {
		closed, open := Folder(tt.name, false), Folder(tt.name, true)
		if closed.Name() != tt.wantClosed {
			t.Errorf("Folder(%q, false) = %q, want %q", tt.name, closed.Name(), tt.wantClosed)
		}
		if open.Name() != tt.wantClosed+openSuffix {
			t.Errorf("Folder(%q, true) = %q, want %q", tt.name, open.Name(), tt.wantClosed+openSuffix)
		}
	}
}

func TestRoot(t *testing.T) {
	if got := Root(false).Name(); got != rootFolderIcon {
		t.Errorf("Root(false) = %q, want %q", got, rootFolderIcon)
	}
	if got := Root(true).Name(); got != rootFolderIcon+openSuffix {
		t.Errorf("Root(true) = %q, want %q", got, rootFolderIcon+openSuffix)
	}
}

func TestLookupsDoNotAllocate(t *testing.T) {
	allocs := testing.AllocsPerRun(100, func() {
		For("README.md")
		For("Some.Component.TEST.ts")
		For("src/Dockerfile")
		ForFile("Unknown.Thing", "Python")
		Folder("Node_Modules", true)
		Folder("__Tests__", false)
		Root(true)
	})
	if allocs != 0 {
		t.Errorf("lookups allocate %v times per run, want 0", allocs)
	}
}

func TestLongAndNonASCIINamesDoNotPanic(t *testing.T) {
	long := strings.Repeat("a", foldLimit*2) + ".GO"
	if got := For(long).Name(); got != "go" {
		t.Errorf("long name = %q, want go", got)
	}
	if got := For("Ünï.GO").Name(); got != "go" {
		t.Errorf("non-ASCII name = %q, want go", got)
	}
	if got := Folder(strings.Repeat("Z", foldLimit*2), false).Name(); got != defaultFolderIcon {
		t.Errorf("long folder = %q, want default", got)
	}
}

func TestTablesPointToDecodableSVGs(t *testing.T) {
	names := map[string]bool{}
	for _, d := range iconDefs {
		names[d.name] = true
		for _, path := range []string{d.dark, d.light} {
			if path == "" {
				continue
			}
			data, err := svgFiles.ReadFile(svgDir + path)
			if err != nil {
				t.Errorf("icon %q: %v", d.name, err)
				continue
			}
			if _, err := ui.ParseSVG(data); err != nil {
				t.Errorf("icon %q (%s) does not decode: %v", d.name, path, err)
			}
		}
		if d.dark == "" {
			t.Errorf("icon %q has no dark SVG", d.name)
		}
	}
	tables := map[string]map[string]string{
		"fileNameIcons":   fileNameIcons,
		"extensionIcons":  extensionIcons,
		"folderNameIcons": folderNameIcons,
		"languageIcons":   languageIcons,
	}
	for table, entries := range tables {
		for key, name := range entries {
			if !names[name] {
				t.Errorf("%s[%q] = %q is not an icon", table, key, name)
			}
			if key != strings.ToLower(key) {
				t.Errorf("%s key %q is not lower case", table, key)
			}
		}
	}
	for _, name := range folderNameIcons {
		if !names[name+openSuffix] {
			t.Errorf("folder icon %q has no open variant", name)
		}
	}
	for _, name := range []string{defaultFileIcon, defaultFolderIcon, defaultFolderIcon + openSuffix, rootFolderIcon, rootFolderIcon + openSuffix} {
		if !names[name] {
			t.Errorf("default icon %q missing", name)
		}
	}
}

func TestEveryEmbeddedFileIsUsed(t *testing.T) {
	used := map[string]bool{}
	for _, d := range iconDefs {
		used[d.dark] = true
		used[d.light] = true
	}
	entries, err := svgFiles.ReadDir("svg")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !used[e.Name()] {
			t.Errorf("embedded %s is not referenced by the table", e.Name())
		}
	}
}

func TestLightVariantFollowsTheme(t *testing.T) {
	icon := For("config.toml")
	if _, path := icon.variant(true); path != "toml.svg" {
		t.Errorf("dark variant = %q, want toml.svg", path)
	}
	if _, path := icon.variant(false); path != "toml_light.svg" {
		t.Errorf("light variant = %q, want toml_light.svg", path)
	}
	plain := For("main.go")
	if _, path := plain.variant(false); path != "go.svg" {
		t.Errorf("icon without light variant = %q, want go.svg", path)
	}
}

func TestEveryIconReachesContrastOnItsSurfaces(t *testing.T) {
	for _, d := range iconDefs {
		for _, dark := range []bool{true, false} {
			path := d.dark
			if !dark && d.light != "" {
				path = d.light
			}
			data, err := svgFiles.ReadFile(svgDir + path)
			if err != nil {
				t.Fatal(err)
			}
			_, required := adaptWithColors(data, dark)
			for _, c := range required {
				if got := worstContrast(c, dark); got < iconContrastFloor {
					t.Errorf("icon %q dark=%v colour %s reaches %.2f:1 on its worst surface, needs %.1f:1", d.name, dark, c.hex(), got, iconContrastFloor)
				}
			}
		}
	}
}

func TestLightVariantDarkensOnlyWhatFails(t *testing.T) {
	for name, want := range map[string]string{"main.go": "#00acc1", "data.json": "#f9a825", "notes.md": "#42a5f5"} {
		icon := For(name)
		data, err := svgFiles.ReadFile(svgDir + icon.darkPath)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(adapt(data, true)); !strings.Contains(got, want) {
			t.Errorf("%s keeps its dark colours %s: %s", name, want, got)
		}
		if got := string(adapt(data, false)); strings.Contains(got, want) {
			t.Errorf("%s light variant still uses %s", name, want)
		}
	}
}

func TestLicenseIsEmbedded(t *testing.T) {
	text := License()
	for _, want := range []string{"Material Extensions", "MIT License", themeCommit} {
		if !strings.Contains(text, want) {
			t.Errorf("license text lacks %q", want)
		}
	}
}

func TestNilIconIsSafe(t *testing.T) {
	var icon *Icon
	if icon.Name() != "" || icon.svg(true) != nil {
		t.Error("nil icon must be empty")
	}
}

func TestViewDrawsAtRequestedSize(t *testing.T) {
	const customSize = 24
	view := func(c *ui.Context) {
		ui.Column(c).Children(func() {
			For("main.go").View(c).Label("default")
			Folder("src", true).ViewSize(c, customSize).Label("custom")
			(*Icon)(nil).View(c).Label("missing")
		})
	}
	for _, dark := range []bool{true, false} {
		tt := ui.NewTesterWith(view, 200, 200, ui.TesterOptions{Dark: dark})
		for label, want := range map[string]float32{"default": DefaultSize, "custom": customSize, "missing": DefaultSize} {
			box, ok := tt.Find(label)
			if !ok {
				t.Errorf("dark=%v %s box not found", dark, label)
				continue
			}
			if box.W != want || box.H != want {
				t.Errorf("dark=%v %s box = %vx%v, want %vx%v", dark, label, box.W, box.H, want, want)
			}
		}
	}
}
