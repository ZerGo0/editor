package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseExportLiterals(t *testing.T) {
	src := `import x from 'y';
// leading comment
export const sample: Sample[] = parseByPattern([
  /* block */
  { name: 'a', light: true, fileExtensions: ['x', "y"], nested: { k: 1.5 }, },
  { name: 'b\'s', enabledFor: [IconPack.Nest], patterns: { eslint: FileNamePattern.Ecmascript } },
]);`
	got, err := parseExport(src, "sample")
	if err != nil {
		t.Fatal(err)
	}
	want := []any{
		object{"name": "a", "light": true, "fileExtensions": []any{"x", "y"}, "nested": object{"k": 1.5}},
		object{
			"name":       "b's",
			"enabledFor": []any{ident("IconPack.Nest")},
			"patterns":   object{"eslint": ident("FileNamePattern.Ecmascript")},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parsed %#v, want %#v", got, want)
	}
}

func TestParseExportErrors(t *testing.T) {
	tests := map[string]string{
		"missing export":   "const other = 1;",
		"unterminated":     "export const sample = [ { name: 'a' ",
		"template":         "export const sample = `a${b}`;",
		"spread":           "export const sample = { ...rest };",
		"unterminated str": "export const sample = 'abc",
		"empty call":       "export const sample = f();",
	}
	for name, src := range tests {
		if _, err := parseExport(src, "sample"); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestParseThemeExpandsPatterns(t *testing.T) {
	sources := themeSources(`{ name: 'eslint', patterns: { eslint: FileNamePattern.Cosmiconfig, tsup: FileNamePattern.Dotfile }, fileNames: ['x'] }`, "", "")
	th, err := parseTheme(sources)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range th.files {
		if e.name == "eslint" {
			names = e.fileNames
		}
	}
	for _, want := range []string{".eslintrc", ".eslintrc.json", "eslint.config.mjs", ".tsup", "tsup", "x"} {
		if !contains(names, want) {
			t.Errorf("pattern names lack %q", want)
		}
	}
}

func TestParseThemeRejectsUnknownPattern(t *testing.T) {
	sources := themeSources(`{ name: 'x', patterns: { a: FileNamePattern.Mystery } }`, "", "")
	if _, err := parseTheme(sources); err == nil || !strings.Contains(err.Error(), "Mystery") {
		t.Errorf("err = %v, want unknown pattern error", err)
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func themeSources(fileEntries, folderEntries, languageEntries string) map[string][]byte {
	return map[string][]byte{
		fileIconsPath: []byte(`export const fileIcons: FileIcons = {
  defaultIcon: { name: 'file' },
  icons: parseByPattern([` + fileEntries + `]),
};`),
		folderIconsPath: []byte(`export const folderIcons: FolderTheme[] = [
  { name: 'specific', defaultIcon: { name: 'folder' }, rootFolder: { name: 'folder-root' }, icons: [` + folderEntries + `] },
  { name: 'none', defaultIcon: { name: '' } },
];`),
		languageIconsPath: []byte("export const languageIcons: LanguageIcon[] = [" + languageEntries + "];"),
		fileGenPath:       []byte("const fileIcon =\n 'M1 1h2'"),
		folderGenPath:     []byte("const folderIcon =\n 'M2 2h2'"),
		openFolderPath:    []byte("const OPEN_FOLDER_PATH =\n 'M3 3h2'"),
		defaultCfgPath:    []byte("const defaultColor = '#90a4ae';"),
		licensePath:       []byte("MIT"),
	}
}
