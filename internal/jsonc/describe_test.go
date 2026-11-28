package jsonc

import (
	"encoding/json"
	"strings"
	"testing"
)

type sample struct {
	Key  string `json:"key"`
	Size int    `json:"size"`
	On   bool   `json:"on"`
}

func TestDescribeTypeErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, in, into, want, field string
	}{
		{"string for number", `"x"`, "float", "expected a number, but found a string", ""},
		{"number for string", `[{"key": 5}]`, "list", "expected a string, but found a number", "0.key"},
		{"fraction for whole number", `[{"size": 1.5}]`, "list", "expected a whole number, but found 1.5", "0.size"},
		{"string for bool", `[{"on": "yes"}]`, "list", "expected true or false, but found a string", "0.on"},
		{"list for object", `[1]`, "list", "expected an object, but found a number", "0"},
		{"object for list", `{"a": 1}`, "list", "expected a list, but found an object", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var err error
			if tt.into == "float" {
				var v float64
				err = json.Unmarshal([]byte(tt.in), &v)
			} else {
				var v []sample
				err = json.Unmarshal([]byte(tt.in), &v)
			}
			f := Describe(err)
			if f.Message != tt.want || f.Field != tt.field || f.Syntax {
				t.Fatalf("Describe = %+v, want %q at %q", f, tt.want, tt.field)
			}
			if strings.Contains(f.Message, "Go ") || strings.Contains(f.Message, "json:") {
				t.Fatalf("message has Go jargon: %q", f.Message)
			}
		})
	}
}

func TestDescribeSyntaxErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, in, want string
		offset         int
	}{
		{"end of file", `{"a": 1`, "unexpected end of file", 7},
		{"bad character", `{"a" 1}`, "unexpected character '1'", 5},
	}
	for _, tt := range tests {
		var v map[string]any
		f := Describe(json.Unmarshal([]byte(tt.in), &v))
		if f.Message != tt.want || f.Offset != tt.offset || !f.Syntax {
			t.Errorf("%s: Describe = %+v, want %q at %d", tt.name, f, tt.want, tt.offset)
		}
	}
}

func TestSentence(t *testing.T) {
	t.Parallel()
	if got := (Failure{Message: "unexpected end of file"}).Sentence(); got != "Unexpected end of file." {
		t.Fatalf("Sentence = %q", got)
	}
}

func TestPosition(t *testing.T) {
	t.Parallel()
	data := []byte("{\n  \"é\": x\n}")
	tests := []struct {
		offset, line, col int
	}{
		{0, 0, 0},
		{2, 1, 0},
		{4, 1, 2},
		{len("{\n  \"é\": "), 1, 7},
		{999, 2, 1},
	}
	for _, tt := range tests {
		if line, col := Position(data, tt.offset); line != tt.line || col != tt.col {
			t.Errorf("Position(%d) = %d,%d want %d,%d", tt.offset, line, col, tt.line, tt.col)
		}
	}
}

func TestAtWithoutOffsetHasNoLine(t *testing.T) {
	t.Parallel()
	p := At([]byte("{}"), -1, "x")
	if p.Located() || p.Line != NoLine {
		t.Fatalf("problem = %+v", p)
	}
	if p := At([]byte("a\nb"), 2, "x"); !p.Located() || p.Line != 1 || p.Col != 0 {
		t.Fatalf("problem = %+v", p)
	}
}

func TestKeyOffsets(t *testing.T) {
	t.Parallel()
	data := []byte("{\n  // note\n  \"a\": 1,\n  \"b\": 2,\n}")
	got := KeyOffsets(data)
	for key, line := range map[string]int{"a": 2, "b": 3} {
		l, c := Position(data, got[key])
		if l != line || c != 2 {
			t.Errorf("%s at %d,%d, want %d,2", key, l, c, line)
		}
	}
	if KeyOffsets([]byte(`{"a": `)) != nil || KeyOffsets([]byte(`[1]`)) != nil {
		t.Error("KeyOffsets accepts a broken or non-object file")
	}
}

func TestElementOffsets(t *testing.T) {
	t.Parallel()
	data := []byte("[\n  {\"key\": \"a\"},\n  // c\n  {\"key\": \"b\"}\n]")
	got := ElementOffsets(data)
	if len(got) != 2 {
		t.Fatalf("offsets = %v", got)
	}
	for i, line := range []int{1, 3} {
		if l, c := Position(data, got[i]); l != line || c != 2 {
			t.Errorf("element %d at %d,%d, want %d,2", i, l, c, line)
		}
	}
	if ElementOffsets([]byte(`{}`)) != nil || ElementOffsets([]byte(`[1,`)) != nil {
		t.Error("ElementOffsets accepts a broken or non-array file")
	}
}
