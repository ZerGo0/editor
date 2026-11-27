package finder

import (
	"cmp"
	"path"
	"slices"
	"strings"
)

const maxFileTypes = 12

type fileType struct {
	name  string
	globs []string
	count int
}

func (d *Dialog) fileTypes() []fileType {
	if d.env.Files == nil {
		return nil
	}
	paths, _ := d.env.Files.Paths()
	if d.types != nil && d.typesFor == len(paths) {
		return d.types
	}
	byExt := map[string]int{}
	sample := map[string]string{}
	for _, p := range paths {
		ext := strings.ToLower(path.Ext(p))
		if ext == "" {
			continue
		}
		byExt[ext]++
		if _, ok := sample[ext]; !ok {
			sample[ext] = p
		}
	}
	byName := map[string]*fileType{}
	for ext, n := range byExt {
		name := ext
		if lang := d.env.Host.Language(sample[ext], ""); lang != nil {
			name = lang.Name
		}
		t := byName[name]
		if t == nil {
			t = &fileType{name: name}
			byName[name] = t
		}
		t.globs = append(t.globs, "*"+ext)
		t.count += n
	}
	types := make([]fileType, 0, len(byName))
	for _, t := range byName {
		slices.Sort(t.globs)
		types = append(types, *t)
	}
	slices.SortFunc(types, func(a, b fileType) int { return cmp.Or(cmp.Compare(b.count, a.count), strings.Compare(a.name, b.name)) })
	d.types, d.typesFor = types[:min(len(types), maxFileTypes)], len(paths)
	if d.types == nil {
		d.types = []fileType{}
	}
	return d.types
}

func (d *Dialog) activeGlobs() []string {
	if len(d.st.FileTypes) == 0 {
		return nil
	}
	var globs []string
	for _, t := range d.fileTypes() {
		if slices.Contains(d.st.FileTypes, t.name) {
			globs = append(globs, t.globs...)
		}
	}
	return globs
}

func (d *Dialog) toggleFileType(name string) {
	if i := slices.Index(d.st.FileTypes, name); i >= 0 {
		d.st.FileTypes = slices.Delete(d.st.FileTypes, i, i+1)
		return
	}
	d.st.FileTypes = append(d.st.FileTypes, name)
}
