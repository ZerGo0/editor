package fileicons

//go:generate go run ./gen

import (
	"bytes"
	"embed"
	"strings"
)

//go:embed svg/*.svg
var svgFiles embed.FS

//go:embed LICENSE-material-icon-theme.md
var license string

const (
	svgDir        = "svg/"
	openSuffix    = "-open"
	foldLimit     = 128
	pathSeparator = '/'
	extSeparator  = '.'
	dunder        = "__"
	folderPrefix  = "._-"
)

type iconDef struct {
	name  string
	dark  string
	light string
}

type folderPair struct {
	closed *Icon
	open   *Icon
}

type index struct {
	fileNames   map[string]*Icon
	extensions  map[string]*Icon
	languages   map[string]*Icon
	folders     map[string]folderPair
	defaultFile *Icon
	folder      folderPair
	root        folderPair
}

var icons = newIndex()

func newIndex() *index {
	byName := make(map[string]*Icon, len(iconDefs))
	for _, d := range iconDefs {
		byName[d.name] = &Icon{name: d.name, darkPath: d.dark, lightPath: d.light}
	}
	pair := func(closed string) folderPair {
		return folderPair{closed: byName[closed], open: byName[closed+openSuffix]}
	}
	x := &index{
		fileNames:   resolve(fileNameIcons, byName),
		extensions:  resolve(extensionIcons, byName),
		languages:   resolve(languageIcons, byName),
		folders:     make(map[string]folderPair, len(folderNameIcons)),
		defaultFile: byName[defaultFileIcon],
		folder:      pair(defaultFolderIcon),
		root:        pair(rootFolderIcon),
	}
	for key, name := range folderNameIcons {
		x.folders[key] = pair(name)
	}
	return x
}

func resolve(table map[string]string, byName map[string]*Icon) map[string]*Icon {
	out := make(map[string]*Icon, len(table))
	for key, name := range table {
		if icon, ok := byName[name]; ok {
			out[key] = icon
		}
	}
	return out
}

func For(name string) *Icon {
	return ForFile(name, "")
}

func ForFile(name, languageID string) *Icon {
	if icon := icons.file(name); icon != nil {
		return icon
	}
	if icon := icons.language(languageID); icon != nil {
		return icon
	}
	return icons.defaultFile
}

func ForLanguage(id string) *Icon {
	if icon := icons.language(id); icon != nil {
		return icon
	}
	return icons.defaultFile
}

func Folder(name string, open bool) *Icon {
	var buf [foldLimit]byte
	key := fold(strings.TrimRight(name, "/"), &buf)
	pair, ok := icons.folderPair(key)
	if !ok {
		pair = icons.folder
	}
	return pair.pick(open)
}

func Root(open bool) *Icon {
	return icons.root.pick(open)
}

func License() string {
	return license
}

func (p folderPair) pick(open bool) *Icon {
	if open {
		return p.open
	}
	return p.closed
}

func (x *index) file(name string) *Icon {
	if cut := strings.LastIndexByte(name, pathSeparator); cut >= 0 {
		name = name[cut+1:]
	}
	var buf [foldLimit]byte
	key := fold(name, &buf)
	if icon, ok := x.fileNames[string(key)]; ok {
		return icon
	}
	for dot := bytes.IndexByte(key, extSeparator); dot >= 0; {
		rest := key[dot+1:]
		if icon, ok := x.extensions[string(rest)]; ok {
			return icon
		}
		next := bytes.IndexByte(rest, extSeparator)
		if next < 0 {
			break
		}
		dot += next + 1
	}
	return nil
}

func (x *index) language(id string) *Icon {
	if id == "" {
		return nil
	}
	var buf [foldLimit]byte
	return x.languages[string(fold(id, &buf))]
}

func (x *index) folderPair(key []byte) (folderPair, bool) {
	if pair, ok := x.folderKey(key); ok {
		return pair, true
	}
	if cut := bytes.LastIndexByte(key, pathSeparator); cut >= 0 {
		return x.folderKey(key[cut+1:])
	}
	return folderPair{}, false
}

func (x *index) folderKey(key []byte) (folderPair, bool) {
	if pair, ok := x.folders[string(key)]; ok {
		return pair, true
	}
	if len(key) > 1 && strings.IndexByte(folderPrefix, key[0]) >= 0 {
		if pair, ok := x.folders[string(key[1:])]; ok {
			return pair, true
		}
	}
	if len(key) > 2*len(dunder) && bytes.HasPrefix(key, []byte(dunder)) && bytes.HasSuffix(key, []byte(dunder)) {
		pair, ok := x.folders[string(key[len(dunder):len(key)-len(dunder)])]
		return pair, ok
	}
	return folderPair{}, false
}

func fold(s string, buf *[foldLimit]byte) []byte {
	if len(s) > foldLimit {
		return []byte(strings.ToLower(s))
	}
	out := buf[:len(s)]
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 0x80 {
			return []byte(strings.ToLower(s))
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return out
}
