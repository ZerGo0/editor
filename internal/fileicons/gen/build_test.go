package main

import (
	"strings"
	"testing"
)

const (
	closedFolderSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16">
  <!-- note -->
  <path id="folder" fill="#fff" d="M0 0h1"/>
  <path id="motive" fill="#000" d="M5 5h1"/>
</svg>`
	plainSVG = `<svg viewBox="0 0 16 16"><path d="M0 0h1"/></svg>`
)

func testSources(t *testing.T) map[string][]byte {
	t.Helper()
	sources := themeSources(`
    { name: 'go', fileExtensions: ['go', 'test.go'], fileNames: ['go.mod', 'a/b'] },
    { name: 'toml', light: true, fileExtensions: ['toml'] },
    { name: 'gated', fileExtensions: ['gated'], enabledFor: [IconPack.Nest] },
    { name: 'skipped', fileExtensions: ['skipped'] },
    { name: 'copy', fileExtensions: ['copy'], clone: { base: 'go', color: 'red-500' } },
  `, `
    { name: 'folder-src', folderNames: ['src', 'github/workflows'], light: true },
    { name: 'folder-clone', folderNames: ['clone'], clone: { base: 'folder-src', color: 'gray-700' } },
  `, `
    { name: 'go', ids: ['go', 'Golang'] },
    { name: 'toml', ids: [] },
  `)
	sources[iconsDir+"go.svg"] = []byte(plainSVG)
	sources[iconsDir+"toml.svg"] = []byte(plainSVG)
	sources[iconsDir+"toml_light.svg"] = []byte(plainSVG)
	sources[iconsDir+"skipped.svg"] = []byte(plainSVG)
	sources[iconsDir+"folder-src.svg"] = []byte(closedFolderSVG)
	sources[iconsDir+"folder-src_light.svg"] = []byte(closedFolderSVG)
	return sources
}

func testAllow(names ...string) map[string]bool {
	allow := map[string]bool{}
	for _, n := range names {
		allow[n] = true
	}
	return allow
}

func TestBuildMapsAllowedIcons(t *testing.T) {
	tbl, err := build(testSources(t), testAllow("go", "toml", "gated", "copy", "folder-src", "folder-clone"))
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name string
		got  map[string]string
		key  string
		want string
	}{
		{"extension", tbl.extensions, "go", "go"},
		{"compound extension", tbl.extensions, "test.go", "go"},
		{"file name", tbl.fileNames, "go.mod", "go"},
		{"clone falls back to base", tbl.extensions, "copy", "go"},
		{"folder", tbl.folders, "src", "folder-src"},
		{"folder path", tbl.folders, "github/workflows", "folder-src"},
		{"folder clone falls back to base", tbl.folders, "clone", "folder-src"},
		{"language", tbl.languages, "go", "go"},
		{"language is lower case", tbl.languages, "golang", "go"},
	}
	for _, c := range checks {
		if got := c.got[c.key]; got != c.want {
			t.Errorf("%s: %q = %q, want %q", c.name, c.key, got, c.want)
		}
	}
	for _, absent := range []string{"gated", "skipped"} {
		if _, ok := tbl.extensions[absent]; ok {
			t.Errorf("extension %q must not be mapped", absent)
		}
	}
	if _, ok := tbl.fileNames["a/b"]; ok {
		t.Error("file name with a path separator must be dropped")
	}
}

func TestBuildProducesOpenAndLightFiles(t *testing.T) {
	tbl, err := build(testSources(t), testAllow("toml", "folder-src"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"toml.svg", "toml_light.svg", "folder-src.svg", "folder-src-open.svg", "folder-src-open_light.svg",
		"file.svg", "folder.svg", "folder-open.svg", "folder-root.svg", "folder-root-open.svg",
	} {
		if _, ok := tbl.files[name]; !ok {
			t.Errorf("file %s missing", name)
		}
	}
	open := string(tbl.files["folder-src-open.svg"])
	if !strings.Contains(open, `d="M3 3h2"`) || !strings.Contains(open, `d="M5 5h1"`) {
		t.Errorf("open folder path not replaced only on the folder: %s", open)
	}
	if strings.Contains(open, "<!--") || strings.Contains(open, "\n") {
		t.Errorf("open folder SVG not minified: %q", open)
	}
	for _, d := range tbl.defs {
		if d.name == "toml" && d.light != "toml_light.svg" {
			t.Errorf("toml light = %q", d.light)
		}
	}
}

func TestBuildErrors(t *testing.T) {
	tests := map[string]struct {
		mutate func(map[string][]byte)
		allow  map[string]bool
	}{
		"unknown allowlist entry": {func(map[string][]byte) {}, testAllow("nope")},
		"missing svg":             {func(s map[string][]byte) { delete(s, iconsDir+"go.svg") }, testAllow("go")},
		"invalid svg":             {func(s map[string][]byte) { s[iconsDir+"go.svg"] = []byte("<svg><path></svg>") }, testAllow("go")},
		"folder without id":       {func(s map[string][]byte) { s[iconsDir+"folder-src.svg"] = []byte(plainSVG) }, testAllow("folder-src")},
		"missing source":          {func(s map[string][]byte) { delete(s, fileGenPath) }, testAllow("go")},
	}
	for name, tt := range tests {
		sources := testSources(t)
		tt.mutate(sources)
		if _, err := build(sources, tt.allow); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestBuildEnforcesSizeBudget(t *testing.T) {
	sources := testSources(t)
	sources[iconsDir+"go.svg"] = []byte(`<svg viewBox="0 0 16 16"><path d="` + strings.Repeat("M0 0h1", maxEmbeddedBytes) + `"/></svg>`)
	if _, err := build(sources, testAllow("go")); err == nil || !strings.Contains(err.Error(), "budget") {
		t.Errorf("err = %v, want budget error", err)
	}
}

func TestMinifyAndValidate(t *testing.T) {
	got := string(minify([]byte("<svg>\n  <!-- c -->\n  <path\n    d=\"M0 0\"/>\n</svg>\n")))
	if want := `<svg><path d="M0 0"/></svg>`; got != want {
		t.Errorf("minify = %q, want %q", got, want)
	}
	if err := validateSVG([]byte("<html></html>")); err == nil {
		t.Error("non-svg root must be rejected")
	}
}

func TestParseAllowlist(t *testing.T) {
	allow, err := parseAllowlist("a\n\n b \n")
	if err != nil || !allow["a"] || !allow["b"] || len(allow) != 2 {
		t.Errorf("allowlist = %v, err %v", allow, err)
	}
	if _, err := parseAllowlist("a\na\n"); err == nil {
		t.Error("duplicate entry must be rejected")
	}
}
