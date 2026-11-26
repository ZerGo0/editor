package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

const (
	pathSeparator    = "/"
	maxEmbeddedBytes = 1_500_000
)

type iconDef struct {
	name  string
	dark  string
	light string
}

type table struct {
	defs       []iconDef
	files      map[string][]byte
	fileNames  map[string]string
	extensions map[string]string
	folders    map[string]string
	languages  map[string]string
	embedded   int
}

type builder struct {
	svgs   map[string][]byte
	allow  map[string]bool
	shapes defaultShapes
	defs   map[string]iconDef
	files  map[string][]byte
}

func build(sources map[string][]byte, allow map[string]bool) (*table, error) {
	th, err := parseTheme(sources)
	if err != nil {
		return nil, fmt.Errorf("parse theme: %w", err)
	}
	if th.defaultFile.name != defaultFileName || th.defaultFolder.name != defaultFolderName || th.rootFolder.name != rootFolderName {
		return nil, fmt.Errorf("default icons are %q, %q, %q: generator expects %q, %q, %q",
			th.defaultFile.name, th.defaultFolder.name, th.rootFolder.name, defaultFileName, defaultFolderName, rootFolderName)
	}
	shapes, err := extractShapes(sources)
	if err != nil {
		return nil, err
	}
	b := &builder{
		svgs:   map[string][]byte{},
		allow:  allow,
		shapes: shapes,
		defs:   map[string]iconDef{},
		files:  map[string][]byte{},
	}
	for name, data := range sources {
		if wanted(name) && strings.HasPrefix(name, iconsDir) {
			b.svgs[strings.TrimPrefix(name, iconsDir)] = data
		}
	}
	t := &table{
		fileNames:  map[string]string{},
		extensions: map[string]string{},
		folders:    map[string]string{},
		languages:  map[string]string{},
	}
	if err := b.addDefaults(); err != nil {
		return nil, err
	}
	if err := b.mapFiles(th, t); err != nil {
		return nil, err
	}
	if err := b.mapFolders(th, t); err != nil {
		return nil, err
	}
	if err := b.mapLanguages(th, t); err != nil {
		return nil, err
	}
	if err := b.checkAllowlist(th); err != nil {
		return nil, err
	}
	t.files = b.files
	t.defs = make([]iconDef, 0, len(b.defs))
	for _, name := range slices.Sorted(maps.Keys(b.defs)) {
		t.defs = append(t.defs, b.defs[name])
	}
	for _, data := range t.files {
		t.embedded += len(data)
	}
	if t.embedded > maxEmbeddedBytes {
		return nil, fmt.Errorf("embedded icons take %d bytes, budget is %d", t.embedded, maxEmbeddedBytes)
	}
	return t, nil
}

func (b *builder) addDefaults() error {
	shapes := b.shapes.svgs()
	for _, name := range slices.Sorted(maps.Keys(shapes)) {
		if err := b.store(name, shapes[name]); err != nil {
			return err
		}
		b.defs[name] = iconDef{name: name, dark: name + svgSuffix}
	}
	return nil
}

func (b *builder) store(name string, data []byte) error {
	data = minify(data)
	if err := validateSVG(data); err != nil {
		return fmt.Errorf("%s%s: %w", name, svgSuffix, err)
	}
	b.files[name+svgSuffix] = data
	return nil
}

func (b *builder) selected(e entry) bool {
	return !e.disabled && !e.gated && b.allow[e.name]
}

func (b *builder) resolve(e entry) (name string, light bool, err error) {
	name = e.name
	if _, ok := b.svgs[name+svgSuffix]; !ok {
		if e.cloneBase == "" {
			return "", false, fmt.Errorf("icon %q has no SVG", e.name)
		}
		name = e.cloneBase
		if _, ok := b.svgs[name+svgSuffix]; !ok {
			return "", false, fmt.Errorf("icon %q: clone base %q has no SVG", e.name, name)
		}
	}
	_, hasLight := b.svgs[name+lightSuffix+svgSuffix]
	return name, e.light && hasLight, nil
}

func (b *builder) useFile(e entry) (string, error) {
	name, light, err := b.resolve(e)
	if err != nil {
		return "", err
	}
	if _, done := b.defs[name]; done {
		return name, nil
	}
	def := iconDef{name: name, dark: name + svgSuffix}
	if err := b.store(name, b.svgs[def.dark]); err != nil {
		return "", err
	}
	if light {
		def.light = name + lightSuffix + svgSuffix
		if err := b.store(name+lightSuffix, b.svgs[def.light]); err != nil {
			return "", err
		}
	}
	b.defs[name] = def
	return name, nil
}

func (b *builder) useFolder(e entry) (string, error) {
	name, light, err := b.resolve(e)
	if err != nil {
		return "", err
	}
	if _, done := b.defs[name]; done {
		return name, nil
	}
	if _, err := b.useFile(e); err != nil {
		return "", err
	}
	open := iconDef{name: name + openSuffix, dark: name + openSuffix + svgSuffix}
	openDark, err := openFolder(b.svgs[name+svgSuffix], b.shapes.openPath)
	if err != nil {
		return "", fmt.Errorf("open variant of %s: %w", name, err)
	}
	if err := b.store(name+openSuffix, openDark); err != nil {
		return "", err
	}
	if light {
		open.light = name + openSuffix + lightSuffix + svgSuffix
		openLight, err := openFolder(b.svgs[name+lightSuffix+svgSuffix], b.shapes.openPath)
		if err != nil {
			return "", fmt.Errorf("open light variant of %s: %w", name, err)
		}
		if err := b.store(name+openSuffix+lightSuffix, openLight); err != nil {
			return "", err
		}
	}
	b.defs[open.name] = open
	return name, nil
}

func usableName(name string) bool {
	return name != "" && !strings.Contains(name, pathSeparator)
}

func (b *builder) mapFiles(th *theme, t *table) error {
	for _, e := range th.files {
		if !b.selected(e) {
			continue
		}
		names := filterUsable(e.fileNames)
		extensions := filterUsable(e.extensions)
		if len(names)+len(extensions) == 0 {
			continue
		}
		icon, err := b.useFile(e)
		if err != nil {
			return err
		}
		for _, n := range names {
			t.fileNames[strings.ToLower(n)] = icon
		}
		for _, x := range extensions {
			t.extensions[strings.ToLower(strings.TrimPrefix(x, "."))] = icon
		}
	}
	return nil
}

func (b *builder) mapFolders(th *theme, t *table) error {
	for _, e := range th.folders {
		if !b.selected(e) {
			continue
		}
		names := e.folders
		if len(names) == 0 {
			continue
		}
		icon, err := b.useFolder(e)
		if err != nil {
			return err
		}
		for _, n := range names {
			t.folders[strings.ToLower(n)] = icon
		}
	}
	return nil
}

func (b *builder) mapLanguages(th *theme, t *table) error {
	for _, e := range th.languages {
		if !b.selected(e) || len(e.ids) == 0 {
			continue
		}
		icon, err := b.useFile(e)
		if err != nil {
			return err
		}
		for _, id := range e.ids {
			t.languages[strings.ToLower(id)] = icon
		}
	}
	return nil
}

func (b *builder) checkAllowlist(th *theme) error {
	known := map[string]bool{}
	for _, group := range [][]entry{th.files, th.folders, th.languages} {
		for _, e := range group {
			known[e.name] = true
		}
	}
	for _, name := range slices.Sorted(maps.Keys(b.allow)) {
		if !known[name] {
			return fmt.Errorf("allowlist entry %q is not an icon of the theme", name)
		}
	}
	return nil
}

func filterUsable(names []string) []string {
	return slices.DeleteFunc(slices.Clone(names), func(n string) bool { return !usableName(n) })
}
