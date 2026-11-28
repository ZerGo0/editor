package jsonc

import (
	"bytes"
	"encoding/json"
)

func KeyOffsets(data []byte) map[string]int {
	if !json.Valid(Standard(data)) {
		return nil
	}
	p := firstValue(data)
	if p >= len(data) || data[p] != '{' {
		return nil
	}
	obj := parseObject(data)
	out := make(map[string]int, len(obj.members))
	for _, m := range obj.members {
		out[m.key] = m.start
	}
	return out
}

func ElementOffsets(data []byte) []int {
	if !json.Valid(Standard(data)) {
		return nil
	}
	p := firstValue(data)
	if p >= len(data) || data[p] != '[' {
		return nil
	}
	var out []int
	p++
	for {
		p = skipTrivia(data, p)
		if p >= len(data) || data[p] == ']' {
			return out
		}
		out = append(out, p)
		p = skipTrivia(data, skipValue(data, p))
		if p < len(data) && data[p] == ',' {
			p++
		}
	}
}

func firstValue(data []byte) int {
	p := 0
	if bytes.HasPrefix(data, []byte(bom)) {
		p = len(bom)
	}
	return skipTrivia(data, p)
}
