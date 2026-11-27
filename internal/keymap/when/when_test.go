package when

import (
	"slices"
	"testing"
)

func TestEval(t *testing.T) {
	ctx := Context{
		"editorTextFocus":          true,
		"editorReadonly":           false,
		"editorLangId":             "markdown",
		"quickDiffDecorationCount": "3",
		"count":                    4,
		"supported":                []string{"go", "markdown"},
	}
	tests := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"editorTextFocus", true},
		{"!editorTextFocus", false},
		{"editorTextFocus && !editorReadonly", true},
		{"editorReadonly || editorTextFocus", true},
		{"editorReadonly || !editorTextFocus", false},
		{"!(editorReadonly || !editorTextFocus)", true},
		{"editorLangId == markdown", true},
		{"editorLangId == 'markdown'", true},
		{"editorLangId === 'markdown'", true},
		{"editorLangId != 'go'", true},
		{"editorLangId !== markdown", false},
		{"quickDiffDecorationCount != '0'", true},
		{"editorLangId =~ /^(markdown|prompt)$/", true},
		{"editorLangId =~ /^MARK/i", true},
		{"editorLangId =~ /^MARK/", false},
		{"count > 3 && count <= 4", true},
		{"count < 3 || count >= 5", false},
		{"editorLangId in supported", true},
		{"editorLangId not in supported", false},
		{"true && editorTextFocus", true},
		{"false || editorReadonly", false},
		{"editorTextFocus && editorReadonly || editorLangId == markdown", true},
	}
	for _, tt := range tests {
		c, err := Parse(tt.in)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.in, err)
			continue
		}
		if got := c.Eval(ctx); got != tt.want {
			t.Errorf("Eval(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestUnknownKeysAreUndefined(t *testing.T) {
	ctx := Context{"editorTextFocus": true}
	for _, tt := range []struct {
		in   string
		want bool
	}{
		{"terminalFocus", false},
		{"!terminalFocus", true},
		{"editorTextFocus && !terminalFocus", true},
		{"editorTextFocus || terminalFocus", true},
		{"terminalFocus && editorTextFocus", false},
		{"debugState == 'running'", false},
		{"debugState != 'running'", true},
		{"neverMatch =~ /doesNotMatch/", false},
	} {
		c, err := Parse(tt.in)
		if err != nil {
			t.Fatal(err)
		}
		if got := c.Eval(ctx); got != tt.want {
			t.Errorf("Eval(%q) = %v, want %v", tt.in, got, tt.want)
		}
		if len(c.Unknown(ctx)) == 0 {
			t.Errorf("Unknown(%q) is empty", tt.in)
		}
	}
}

func TestKeys(t *testing.T) {
	c, err := Parse("b && a || !b && c in list")
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Keys(); !slices.Equal(got, []string{"a", "b", "c", "list"}) {
		t.Errorf("Keys = %v", got)
	}
	if c.String() != "b && a || !b && c in list" {
		t.Errorf("String = %q", c.String())
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{"a &&", "(a", "a == ", "a == 'x", "a =~ x", "a =~ /x", "a =~ /(/", "&& a", "a b", "a not b", "a in"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) = nil error", in)
		}
	}
}

func TestNilClause(t *testing.T) {
	var c *Clause
	if !c.Eval(nil) || c.Keys() != nil || c.String() != "" {
		t.Error("a nil clause must always be true")
	}
}

func TestNormalize(t *testing.T) {
	if Normalize(" a &&  !b ") != Normalize("a&&!b") {
		t.Error("Normalize must ignore whitespace")
	}
}
