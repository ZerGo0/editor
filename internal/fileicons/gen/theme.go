package main

import (
	"fmt"
	"maps"
	"slices"
)

const specificFolderTheme = "specific"

type entry struct {
	name       string
	light      bool
	disabled   bool
	gated      bool
	cloneBase  string
	extensions []string
	fileNames  []string
	folders    []string
	ids        []string
}

type theme struct {
	defaultFile   entry
	files         []entry
	defaultFolder entry
	rootFolder    entry
	folders       []entry
	languages     []entry
}

const (
	ecmascriptPattern    = "FileNamePattern.Ecmascript"
	configurationPattern = "FileNamePattern.Configuration"
	nodeEcosystemPattern = "FileNamePattern.NodeEcosystem"
	yamlPattern          = "FileNamePattern.Yaml"
	dotfilePattern       = "FileNamePattern.Dotfile"
	cosmiconfigPattern   = "FileNamePattern.Cosmiconfig"
)

var patternSuffixes = map[string][]string{
	ecmascriptPattern:    {".js", ".mjs", ".cjs", ".ts", ".mts", ".cts"},
	configurationPattern: {".json", ".jsonc", ".json5", ".yaml", ".yml", ".toml"},
	nodeEcosystemPattern: {".js", ".mjs", ".cjs", ".ts", ".mts", ".cts", ".json", ".jsonc", ".json5", ".yaml", ".yml", ".toml"},
	yamlPattern:          {".yaml", ".yml"},
}

func parseTheme(sources map[string][]byte) (*theme, error) {
	t := &theme{}

	filesRoot, err := exportObject(sources, fileIconsPath, "fileIcons")
	if err != nil {
		return nil, err
	}
	if t.defaultFile, err = decodeEntry(filesRoot["defaultIcon"]); err != nil {
		return nil, fmt.Errorf("fileIcons defaultIcon: %w", err)
	}
	if t.files, err = decodeEntries(filesRoot["icons"]); err != nil {
		return nil, fmt.Errorf("fileIcons icons: %w", err)
	}

	folderThemes, err := exportValue(sources, folderIconsPath, "folderIcons")
	if err != nil {
		return nil, err
	}
	specific, err := findTheme(folderThemes, specificFolderTheme)
	if err != nil {
		return nil, err
	}
	if t.defaultFolder, err = decodeEntry(specific["defaultIcon"]); err != nil {
		return nil, fmt.Errorf("folder defaultIcon: %w", err)
	}
	if t.rootFolder, err = decodeEntry(specific["rootFolder"]); err != nil {
		return nil, fmt.Errorf("folder rootFolder: %w", err)
	}
	if t.folders, err = decodeEntries(specific["icons"]); err != nil {
		return nil, fmt.Errorf("folder icons: %w", err)
	}

	languages, err := exportValue(sources, languageIconsPath, "languageIcons")
	if err != nil {
		return nil, err
	}
	if t.languages, err = decodeEntries(languages); err != nil {
		return nil, fmt.Errorf("languageIcons: %w", err)
	}
	return t, nil
}

func exportValue(sources map[string][]byte, path, name string) (any, error) {
	data, ok := sources[path]
	if !ok {
		return nil, fmt.Errorf("source %s missing", path)
	}
	return parseExport(string(data), name)
}

func exportObject(sources map[string][]byte, path, name string) (object, error) {
	value, err := exportValue(sources, path, name)
	if err != nil {
		return nil, err
	}
	obj, ok := value.(object)
	if !ok {
		return nil, fmt.Errorf("export %q is not an object", name)
	}
	return obj, nil
}

func findTheme(value any, name string) (object, error) {
	themes, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("folderIcons is not an array")
	}
	for _, item := range themes {
		if obj, ok := item.(object); ok && obj["name"] == name {
			return obj, nil
		}
	}
	return nil, fmt.Errorf("folder theme %q not found", name)
}

func decodeEntries(value any) ([]entry, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("not an array")
	}
	entries := make([]entry, 0, len(items))
	for i, item := range items {
		e, err := decodeEntry(item)
		if err != nil {
			return nil, fmt.Errorf("item %d: %w", i, err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func decodeEntry(value any) (entry, error) {
	obj, ok := value.(object)
	if !ok {
		return entry{}, fmt.Errorf("not an object")
	}
	var e entry
	var err error
	if e.name, err = stringField(obj, "name"); err != nil {
		return entry{}, err
	}
	e.light, _ = obj["light"].(bool)
	e.disabled, _ = obj["disabled"].(bool)
	if clone, ok := obj["clone"].(object); ok {
		if e.cloneBase, err = stringField(clone, "base"); err != nil {
			return entry{}, fmt.Errorf("%s clone: %w", e.name, err)
		}
	}
	packs, err := stringsField(obj, "enabledFor")
	if err != nil {
		return entry{}, fmt.Errorf("%s: %w", e.name, err)
	}
	e.gated = len(packs) > 0
	if e.extensions, err = stringsField(obj, "fileExtensions"); err != nil {
		return entry{}, fmt.Errorf("%s: %w", e.name, err)
	}
	if e.fileNames, err = stringsField(obj, "fileNames"); err != nil {
		return entry{}, fmt.Errorf("%s: %w", e.name, err)
	}
	if e.folders, err = stringsField(obj, "folderNames"); err != nil {
		return entry{}, fmt.Errorf("%s: %w", e.name, err)
	}
	if e.ids, err = stringsField(obj, "ids"); err != nil {
		return entry{}, fmt.Errorf("%s: %w", e.name, err)
	}
	patterns, err := patternNames(obj["patterns"])
	if err != nil {
		return entry{}, fmt.Errorf("%s: %w", e.name, err)
	}
	e.fileNames = append(patterns, e.fileNames...)
	return e, nil
}

func stringField(obj object, key string) (string, error) {
	s, ok := obj[key].(string)
	if !ok || s == "" {
		return "", fmt.Errorf("field %q is not a string", key)
	}
	return s, nil
}

func stringsField(obj object, key string) ([]string, error) {
	value, present := obj[key]
	if !present {
		return nil, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("field %q is not an array", key)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case string:
			out = append(out, v)
		case ident:
			out = append(out, string(v))
		default:
			return nil, fmt.Errorf("field %q has a non-string item", key)
		}
	}
	return out, nil
}

func patternNames(value any) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	obj, ok := value.(object)
	if !ok {
		return nil, fmt.Errorf("patterns is not an object")
	}
	var names []string
	for _, base := range slices.Sorted(maps.Keys(obj)) {
		kind, ok := obj[base].(ident)
		if !ok {
			return nil, fmt.Errorf("pattern %q is not an identifier", base)
		}
		expanded, err := expandPattern(base, string(kind))
		if err != nil {
			return nil, err
		}
		names = append(names, expanded...)
	}
	return names, nil
}

func expandPattern(base, kind string) ([]string, error) {
	switch kind {
	case dotfilePattern:
		return []string{"." + base, base}, nil
	case cosmiconfigPattern:
		return cosmiconfigNames(base), nil
	}
	suffixes, ok := patternSuffixes[kind]
	if !ok {
		return nil, fmt.Errorf("unknown pattern %q", kind)
	}
	names := make([]string, 0, len(suffixes))
	for _, suffix := range suffixes {
		names = append(names, base+suffix)
	}
	return names, nil
}

func cosmiconfigNames(base string) []string {
	nodeSuffixes := patternSuffixes[nodeEcosystemPattern]
	var names []string
	for _, prefix := range []string{"." + base + "rc", ".config/" + base + "rc"} {
		names = append(names, prefix)
		for _, suffix := range nodeSuffixes {
			names = append(names, prefix+suffix)
		}
	}
	for _, suffix := range nodeSuffixes {
		names = append(names, base+".config"+suffix)
	}
	return names
}
