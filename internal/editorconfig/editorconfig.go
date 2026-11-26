package editorconfig

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	FileName       = ".editorconfig"
	commentHash    = '#'
	commentSemi    = ';'
	sectionOpen    = '['
	sectionClose   = ']'
	pairSeparator  = "="
	keyRoot        = "root"
	valueTrue      = "true"
	valueFalse     = "false"
	valueUnset     = "unset"
	valueTab       = "tab"
	keyIndentStyle = "indent_style"
	keyIndentSize  = "indent_size"
	keyTabWidth    = "tab_width"
	keyEndOfLine   = "end_of_line"
	keyCharset     = "charset"
	keyTrim        = "trim_trailing_whitespace"
	keyFinal       = "insert_final_newline"
	keyMaxLine     = "max_line_length"

	IndentTabs   = "tab"
	IndentSpaces = "space"
	CharsetUTF8  = "utf-8"
)

var (
	errInvalidLine = errors.New("not a section, a property or a comment")
	lineEndings    = map[string]string{"lf": "\n", "crlf": "\r\n", "cr": "\r"}
)

type Section struct {
	Glob  string
	Props map[string]string
}

type File struct {
	Root     bool
	Sections []Section
}

type Props map[string]string

type Toggle uint8

const (
	Unset Toggle = iota
	On
	Off
)

type Settings struct {
	IndentStyle            string
	IndentSize             int
	TabWidth               int
	EndOfLine              string
	Charset                string
	TrimTrailingWhitespace Toggle
	InsertFinalNewline     Toggle
	MaxLineLength          int
}

func Parse(data []byte) (File, error) {
	var f File
	var errs []error
	scanner := bufio.NewScanner(bytes.NewReader(data))
	n := 0
	for scanner.Scan() {
		n++
		line := strings.TrimSpace(scanner.Text())
		if n == 1 {
			line = strings.TrimPrefix(line, "\ufeff")
		}
		switch {
		case line == "" || line[0] == commentHash || line[0] == commentSemi:
		case line[0] == sectionOpen && line[len(line)-1] == sectionClose:
			f.Sections = append(f.Sections, Section{Glob: line[1 : len(line)-1], Props: map[string]string{}})
		default:
			key, value, ok := strings.Cut(line, pairSeparator)
			if !ok {
				errs = append(errs, fmt.Errorf("line %d: %w", n, errInvalidLine))
				continue
			}
			key, value = strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(value)
			if len(f.Sections) == 0 {
				if key == keyRoot {
					f.Root = strings.EqualFold(value, valueTrue)
				}
				continue
			}
			f.Sections[len(f.Sections)-1].Props[key] = strings.ToLower(value)
		}
	}
	if err := scanner.Err(); err != nil {
		errs = append(errs, err)
	}
	return f, errors.Join(errs...)
}

func Resolve(path string, read func(string) ([]byte, error)) (Props, []string, error) {
	type found struct {
		dir  string
		file File
	}
	var chain []found
	var consulted []string
	var errs []error
	for dir := filepath.Dir(path); ; {
		name := filepath.Join(dir, FileName)
		consulted = append(consulted, name)
		data, err := read(name)
		switch {
		case err == nil:
			f, perr := Parse(data)
			if perr != nil {
				errs = append(errs, fmt.Errorf("%s: %w", name, perr))
			}
			chain = append(chain, found{dir, f})
			if f.Root {
				dir = ""
			}
		case !errors.Is(err, fs.ErrNotExist):
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
		parent := filepath.Dir(dir)
		if dir == "" || parent == dir {
			break
		}
		dir = parent
	}
	props := Props{}
	for i := len(chain) - 1; i >= 0; i-- {
		rel, err := filepath.Rel(chain[i].dir, path)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		for _, s := range chain[i].file.Sections {
			if Match(s.Glob, rel) {
				for k, v := range s.Props {
					props[k] = v
				}
			}
		}
	}
	for k, v := range props {
		if v == valueUnset {
			delete(props, k)
		}
	}
	return props, consulted, errors.Join(errs...)
}

func (p Props) Settings() Settings {
	s := Settings{
		Charset:                p[keyCharset],
		TrimTrailingWhitespace: toggle(p[keyTrim]),
		InsertFinalNewline:     toggle(p[keyFinal]),
		MaxLineLength:          positive(p[keyMaxLine]),
		TabWidth:               positive(p[keyTabWidth]),
		IndentSize:             positive(p[keyIndentSize]),
	}
	if style := p[keyIndentStyle]; style == IndentTabs || style == IndentSpaces {
		s.IndentStyle = style
	}
	if end, ok := lineEndings[p[keyEndOfLine]]; ok {
		s.EndOfLine = end
	}
	if p[keyIndentSize] == valueTab || s.IndentSize == 0 && s.IndentStyle == IndentTabs {
		s.IndentSize = s.TabWidth
	}
	if s.TabWidth == 0 {
		s.TabWidth = s.IndentSize
	}
	return s
}

func (s Settings) IsZero() bool {
	return s == Settings{}
}

func toggle(v string) Toggle {
	switch v {
	case valueTrue:
		return On
	case valueFalse:
		return Off
	}
	return Unset
}

func positive(v string) int {
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}
