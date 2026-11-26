package jsonc

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func checkResult(t *testing.T, got []byte, want string, wantMap map[string]any) {
	t.Helper()
	std := Standard(got)
	if !json.Valid(std) {
		t.Fatalf("invalid JSON after edit:\n%s", got)
	}
	var m map[string]any
	if err := json.Unmarshal(std, &m); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m, wantMap) {
		t.Fatalf("map = %v, want %v\n%s", m, wantMap, got)
	}
	if want != "" && string(got) != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestStandard(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"line comment", "{\"a\": 1 // c\n}", "{\"a\": 1     \n}"},
		{"block comment keeps newlines", "{/* a\nb */}", "{    \n    }"},
		{"string untouched", `{"a":"//x /* y */"}`, `{"a":"//x /* y */"}`},
		{"escaped quote", `{"a":"\" // z"}`, `{"a":"\" // z"}`},
		{"trailing commas", `{"a":[1,], "b":2, }`, `{"a":[1 ], "b":2  }`},
		{"comma before comment before brace", "{\"a\":1, // c\n}", "{\"a\":1      \n}"},
		{"bom", "\xef\xbb\xbf{}", "   {}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Standard([]byte(tt.in))
			if string(got) != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
			if len(got) != len(tt.in) {
				t.Fatalf("length changed: %d != %d", len(got), len(tt.in))
			}
		})
	}
}

func TestSetKey(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		key   string
		value string
		want  string
		m     map[string]any
	}{
		{"empty file", "", "a", "1", "{\n\t\"a\": 1\n}\n", map[string]any{"a": 1.0}},
		{"whitespace file", "  \n", "a", "1", "  \n{\n\t\"a\": 1\n}\n", map[string]any{"a": 1.0}},
		{"only comment", "// hi\n", "a", "1", "// hi\n{\n\t\"a\": 1\n}\n", map[string]any{"a": 1.0}},
		{"only block comment no newline", "/* hi */", "a", "1", "/* hi */\n{\n\t\"a\": 1\n}\n", map[string]any{"a": 1.0}},
		{"empty object", "{}", "a", "1", "{\n\t\"a\": 1\n}", map[string]any{"a": 1.0}},
		{"empty object newline", "{\n}\n", "a", "1", "{\n\t\"a\": 1\n}\n", map[string]any{"a": 1.0}},
		{"append detects indent", "{\n  \"x\": 1\n}\n", "a", "1", "{\n  \"x\": 1,\n  \"a\": 1\n}\n", map[string]any{"x": 1.0, "a": 1.0}},
		{"append after trailing comma", "{\n\t\"x\": 1,\n}\n", "a", "1", "{\n\t\"x\": 1,\n\t\"a\": 1\n}\n", map[string]any{"x": 1.0, "a": 1.0}},
		{
			"append keeps comments",
			"{\n\t// c\n\t\"x\": 1 // note\n\t// end\n}\n", "a", "1",
			"{\n\t// c\n\t\"x\": 1, // note\n\t\"a\": 1\n\t// end\n}\n",
			map[string]any{"x": 1.0, "a": 1.0},
		},
		{"append inline", `{"x": 1}`, "a", "2", "{\"x\": 1,\n\t\"a\": 2\n}", map[string]any{"x": 1.0, "a": 2.0}},
		{
			"append multi-line value",
			"{\n  \"x\": 1\n}\n", "a", "{\n  \"q\": [\n    1\n  ]\n}",
			"{\n  \"x\": 1,\n  \"a\": {\n    \"q\": [\n      1\n    ]\n  }\n}\n",
			map[string]any{"x": 1.0, "a": map[string]any{"q": []any{1.0}}},
		},
		{
			"replace multi-line value",
			"{\n\t\"x\": {\n\t\t\"y\": 1\n\t},\n\t\"z\": 2\n}", "x", "{\n\t\"q\": [1, 2]\n}",
			"{\n\t\"x\": {\n\t\t\"q\": [1, 2]\n\t},\n\t\"z\": 2\n}",
			map[string]any{"x": map[string]any{"q": []any{1.0, 2.0}}, "z": 2.0},
		},
		{
			"replace with escaped key and value",
			"{\"a\\\"b\": \"v\\\"1\" // c\n}", "a\"b", `"new"`,
			"{\"a\\\"b\": \"new\" // c\n}",
			map[string]any{"a\"b": "new"},
		},
		{
			"comment after value kept on replace",
			"{\n\t\"a\": 1, /* x */\n\t\"b\": [1, 2,], // y\n}", "b", "true",
			"{\n\t\"a\": 1, /* x */\n\t\"b\": true, // y\n}",
			map[string]any{"a": 1.0, "b": true},
		},
		{"comment markers in string", `{"url": "http://x" /* c */}`, "a", "1", "", map[string]any{"url": "http://x", "a": 1.0}},
		{"bom", "\xef\xbb\xbf{\n\t\"x\": 1\n}\n", "a", "1", "", map[string]any{"x": 1.0, "a": 1.0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SetKey([]byte(tt.in), tt.key, []byte(tt.value))
			if err != nil {
				t.Fatal(err)
			}
			checkResult(t, got, tt.want, tt.m)
		})
	}
}

func TestDeleteKey(t *testing.T) {
	three := "{\n\t\"a\": 1,\n\t\"b\": 2,\n\t\"c\": 3\n}\n"
	tests := []struct {
		name string
		in   string
		key  string
		want string
		m    map[string]any
	}{
		{"first", three, "a", "{\n\t\"b\": 2,\n\t\"c\": 3\n}\n", map[string]any{"b": 2.0, "c": 3.0}},
		{"middle", three, "b", "{\n\t\"a\": 1,\n\t\"c\": 3\n}\n", map[string]any{"a": 1.0, "c": 3.0}},
		{"last", three, "c", "{\n\t\"a\": 1,\n\t\"b\": 2\n}\n", map[string]any{"a": 1.0, "b": 2.0}},
		{"only", "{\n\t\"a\": 1\n}\n", "a", "{\n}\n", map[string]any{}},
		{"last with trailing comma", "{\n\t\"a\": 1,\n\t\"b\": 2,\n}", "b", "{\n\t\"a\": 1,\n}", map[string]any{"a": 1.0}},
		{
			"line comments",
			"{\n\t\"a\": 1, // one\n\t\"b\": 2 // two\n}", "b",
			"{\n\t\"a\": 1 // one\n}", map[string]any{"a": 1.0},
		},
		{
			"leading comment stays",
			"{\n\t// about a\n\t\"a\": 1,\n\t\"b\": 2\n}", "a",
			"{\n\t// about a\n\t\"b\": 2\n}", map[string]any{"b": 2.0},
		},
		{"inline first", `{"a": 1, "b": 2}`, "a", `{"b": 2}`, map[string]any{"b": 2.0}},
		{"inline last", `{"a": 1, "b": 2}`, "b", `{"a": 1 }`, map[string]any{"a": 1.0}},
		{
			"nested multi-line value",
			"{\n\t\"a\": {\n\t\t\"x\": [1,\n2]\n\t},\n\t\"b\": 2\n}", "a",
			"{\n\t\"b\": 2\n}", map[string]any{"b": 2.0},
		},
		{"escaped key", `{"a\"b": 1, "c": 2}`, "a\"b", `{"c": 2}`, map[string]any{"c": 2.0}},
		{"nested key ignored", `{"a": {"b": 1}}`, "b", `{"a": {"b": 1}}`, map[string]any{"a": map[string]any{"b": 1.0}}},
		{"missing", three, "zzz", three, map[string]any{"a": 1.0, "b": 2.0, "c": 3.0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DeleteKey([]byte(tt.in), tt.key)
			if err != nil {
				t.Fatal(err)
			}
			checkResult(t, got, tt.want, tt.m)
		})
	}
}

func TestDeleteKeyEmptyDocument(t *testing.T) {
	for _, in := range []string{"", "// only\n"} {
		got, err := DeleteKey([]byte(in), "a")
		if err != nil || string(got) != in {
			t.Fatalf("got %q, %v", got, err)
		}
	}
}

func TestMalformed(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantLine string
	}{
		{"array", "[1]", "line 1"},
		{"string", "\n\"x\"", "line 2"},
		{"missing value", `{"a":`, "line 1"},
		{"missing colon", `{"a" 1}`, "line 1"},
		{"extra brace", `{"a":1}}`, "line 1"},
		{"double comma", "{\n\"a\": 1,\n,}", "line 3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := SetKey([]byte(tt.in), "k", []byte("1")); err == nil || !strings.Contains(err.Error(), tt.wantLine) {
				t.Fatalf("SetKey error = %v, want %s", err, tt.wantLine)
			}
			if _, err := DeleteKey([]byte(tt.in), "k"); err == nil || !strings.Contains(err.Error(), tt.wantLine) {
				t.Fatalf("DeleteKey error = %v, want %s", err, tt.wantLine)
			}
		})
	}
}
