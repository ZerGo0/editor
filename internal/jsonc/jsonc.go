package jsonc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	bom               = "\xef\xbb\xbf"
	defaultIndent     = "\t"
	lineCommentStart  = "//"
	blockCommentStart = "/*"
	blockCommentEnd   = "*/"
	inlineSpaces      = " \t"
	allSpaces         = " \t\r\n"
	scalarEnd         = allSpaces + ",}]/"
	noIndex           = -1
)

type member struct {
	key      string
	start    int
	valStart int
	valEnd   int
	comma    int
}

type object struct {
	open    int
	close   int
	members []member
}

func (o object) find(key string) int {
	for i := len(o.members) - 1; i >= 0; i-- {
		if o.members[i].key == key {
			return i
		}
	}
	return noIndex
}

func Standard(data []byte) []byte {
	out := bytes.Clone(data)
	if bytes.HasPrefix(out, []byte(bom)) {
		blank(out[:len(bom)])
	}
	lastComma := noIndex
	for i := 0; i < len(out); {
		switch c := out[i]; c {
		case '"':
			i = skipString(out, i)
			lastComma = noIndex
		case ',':
			lastComma = i
			i++
		case '}', ']':
			if lastComma != noIndex {
				out[lastComma] = ' '
			}
			lastComma = noIndex
			i++
		case '/':
			end := skipTrivia(out, i)
			if end == i {
				i++
				continue
			}
			blank(out[i:end])
			i = end
		default:
			if bytes.IndexByte([]byte(allSpaces), c) == noIndex {
				lastComma = noIndex
			}
			i++
		}
	}
	return out
}

func SetKey(data []byte, key string, value []byte) ([]byte, error) {
	obj, empty, err := load(data)
	if err != nil {
		return nil, err
	}
	if empty {
		return newDocument(data, key, value), nil
	}
	if i := obj.find(key); i != noIndex {
		m := obj.members[i]
		indent, _ := lineIndent(data, m.start)
		return splice(data, m.valStart, m.valEnd, reindent(value, indent)), nil
	}
	return insertMember(data, obj, key, value), nil
}

func DeleteKey(data []byte, key string) ([]byte, error) {
	for {
		obj, empty, err := load(data)
		if err != nil {
			return nil, err
		}
		if empty {
			return data, nil
		}
		i := obj.find(key)
		if i == noIndex {
			return data, nil
		}
		data = removeMember(data, obj, i)
	}
}

func load(data []byte) (object, bool, error) {
	std := Standard(data)
	if len(bytes.TrimSpace(std)) == 0 {
		return object{}, true, nil
	}
	var v any
	if err := json.Unmarshal(std, &v); err != nil {
		var syntax *json.SyntaxError
		if errors.As(err, &syntax) {
			return object{}, false, fmt.Errorf("line %d: %w", lineAt(data, int(syntax.Offset)-1), err)
		}
		return object{}, false, err
	}
	if _, ok := v.(map[string]any); !ok {
		first := len(std) - len(bytes.TrimLeft(std, allSpaces))
		return object{}, false, fmt.Errorf("line %d: top-level value must be an object", lineAt(data, first))
	}
	return parseObject(data), false, nil
}

func parseObject(data []byte) object {
	p := 0
	if bytes.HasPrefix(data, []byte(bom)) {
		p = len(bom)
	}
	p = skipTrivia(data, p)
	obj := object{open: p}
	p++
	for {
		p = skipTrivia(data, p)
		if data[p] == '}' {
			obj.close = p
			return obj
		}
		m := member{start: p, comma: noIndex}
		keyEnd := skipString(data, p)
		_ = json.Unmarshal(data[p:keyEnd], &m.key)
		p = skipTrivia(data, keyEnd)
		p = skipTrivia(data, p+1)
		m.valStart = p
		m.valEnd = skipValue(data, p)
		p = skipTrivia(data, m.valEnd)
		if data[p] == ',' {
			m.comma = p
			p++
		}
		obj.members = append(obj.members, m)
	}
}

func newDocument(data []byte, key string, value []byte) []byte {
	out := bytes.Clone(data)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	out = append(out, "{\n"...)
	out = append(out, defaultIndent...)
	out = append(out, memberText(key, value, defaultIndent)...)
	return append(out, "\n}\n"...)
}

func insertMember(data []byte, obj object, key string, value []byte) []byte {
	indent := defaultIndent
	restStart := obj.open + 1
	separator := ""
	if len(obj.members) > 0 {
		if found, ok := lineIndent(data, obj.members[0].start); ok {
			indent = found
		}
		last := obj.members[len(obj.members)-1]
		if last.comma != noIndex {
			restStart = last.comma + 1
		} else {
			restStart = last.valEnd
			separator = ","
		}
	}
	rest := data[restStart:]
	text := memberText(key, value, indent)
	out := append(bytes.Clone(data[:restStart]), separator...)
	if eol := lineEnd(rest); eol != noIndex {
		out = append(out, rest[:eol]...)
		out = append(out, "\n"+indent+text...)
		return append(out, rest[eol:]...)
	}
	out = append(out, "\n"+indent+text+"\n"...)
	return append(out, bytes.TrimLeft(rest, inlineSpaces)...)
}

func removeMember(data []byte, obj object, i int) []byte {
	m := obj.members[i]
	after := m.valEnd
	if m.comma != noIndex {
		after = m.comma + 1
	}
	lineStart := bytes.LastIndexByte(data[:m.start], '\n') + 1
	start, end := m.start, after
	if eol := lineEnd(data[after:]); eol != noIndex && len(bytes.TrimSpace(data[lineStart:m.start])) == 0 {
		start, end = lineStart, after+eol+1
	} else {
		for end < len(data) && bytes.IndexByte([]byte(inlineSpaces), data[end]) != noIndex {
			end++
		}
	}
	out := splice(data, start, end, nil)
	if i > 0 && i == len(obj.members)-1 && m.comma == noIndex {
		if prev := obj.members[i-1].comma; prev != noIndex {
			out = splice(out, prev, prev+1, nil)
		}
	}
	return out
}

func memberText(key string, value []byte, indent string) string {
	quoted, _ := json.Marshal(key)
	return string(quoted) + ": " + string(reindent(value, indent))
}

func reindent(value []byte, indent string) []byte {
	return bytes.ReplaceAll(bytes.TrimSpace(value), []byte("\n"), []byte("\n"+indent))
}

func lineIndent(data []byte, offset int) (string, bool) {
	lineStart := bytes.LastIndexByte(data[:offset], '\n') + 1
	end := lineStart
	for end < offset && bytes.IndexByte([]byte(inlineSpaces), data[end]) != noIndex {
		end++
	}
	return string(data[lineStart:end]), end == offset
}

func lineEnd(rest []byte) int {
	i := 0
	for i < len(rest) && bytes.IndexByte([]byte(inlineSpaces+"\r"), rest[i]) != noIndex {
		i++
	}
	if i == len(rest) {
		return noIndex
	}
	if rest[i] == '\n' {
		return i
	}
	if bytes.HasPrefix(rest[i:], []byte(lineCommentStart)) {
		if j := bytes.IndexByte(rest[i:], '\n'); j != noIndex {
			return i + j
		}
	}
	return noIndex
}

func splice(data []byte, start, end int, replacement []byte) []byte {
	out := make([]byte, 0, len(data)-(end-start)+len(replacement))
	out = append(out, data[:start]...)
	out = append(out, replacement...)
	return append(out, data[end:]...)
}

func lineAt(data []byte, offset int) int {
	offset = max(0, min(offset, len(data)))
	return 1 + bytes.Count(data[:offset], []byte("\n"))
}

func blank(b []byte) {
	for i, c := range b {
		if c != '\n' && c != '\r' {
			b[i] = ' '
		}
	}
}

func skipString(data []byte, p int) int {
	for p++; p < len(data); p++ {
		switch data[p] {
		case '\\':
			p++
		case '"':
			return p + 1
		}
	}
	return len(data)
}

func skipTrivia(data []byte, p int) int {
	for p < len(data) {
		switch {
		case bytes.IndexByte([]byte(allSpaces), data[p]) != noIndex:
			p++
		case bytes.HasPrefix(data[p:], []byte(lineCommentStart)):
			end := bytes.IndexByte(data[p:], '\n')
			if end == noIndex {
				return len(data)
			}
			p += end
		case bytes.HasPrefix(data[p:], []byte(blockCommentStart)):
			end := bytes.Index(data[p+len(blockCommentStart):], []byte(blockCommentEnd))
			if end == noIndex {
				return len(data)
			}
			p += len(blockCommentStart) + end + len(blockCommentEnd)
		default:
			return p
		}
	}
	return p
}

func skipValue(data []byte, p int) int {
	switch data[p] {
	case '"':
		return skipString(data, p)
	case '{', '[':
		depth := 0
		for i := p; i < len(data); {
			switch data[i] {
			case '"':
				i = skipString(data, i)
				continue
			case '/':
				if next := skipTrivia(data, i); next > i {
					i = next
					continue
				}
			case '{', '[':
				depth++
			case '}', ']':
				depth--
				if depth == 0 {
					return i + 1
				}
			}
			i++
		}
		return len(data)
	}
	for p < len(data) && bytes.IndexByte([]byte(scalarEnd), data[p]) == noIndex {
		p++
	}
	return p
}
