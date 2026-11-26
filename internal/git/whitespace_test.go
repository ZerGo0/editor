package git

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestWhitespaceModeFlags(t *testing.T) {
	t.Parallel()
	cases := []struct {
		mode WhitespaceMode
		want []string
	}{
		{WhitespaceNone, nil},
		{WhitespaceTrim, []string{"--ignore-space-at-eol"}},
		{WhitespaceIgnore, []string{"-w"}},
		{WhitespaceIgnoreBlank, []string{"-w", "--ignore-blank-lines"}},
		{WhitespaceMode(99), nil},
	}
	for _, c := range cases {
		if got := c.mode.Flags(); !slices.Equal(got, c.want) {
			t.Errorf("mode %d flags = %v, want %v", c.mode, got, c.want)
		}
	}
}

func TestWhitespaceModeKeysRoundTrip(t *testing.T) {
	t.Parallel()
	for _, e := range whitespaceModes {
		got, ok := ParseWhitespaceMode(e.mode.Key())
		if !ok || got != e.mode {
			t.Errorf("key %q parsed to %d, %v", e.mode.Key(), got, ok)
		}
	}
	if _, ok := ParseWhitespaceMode("bogus"); ok {
		t.Error("an unknown key was accepted")
	}
	if got := WhitespaceMode(99).Key(); got != "none" {
		t.Errorf("unknown mode key = %q", got)
	}
}

func TestDiffTextsHonorsMode(t *testing.T) {
	t.Parallel()
	if !Available() {
		t.Skip("git is not installed")
	}
	old := "func f() {\n\tx := 1\n\treturn x\n}\n"
	next := "func f() {\n    x := 1\n    return x\n}\n"
	ctx := context.Background()
	plain, err := DiffTexts(ctx, old, next, WhitespaceNone)
	if err != nil || !strings.Contains(string(plain), "+    x := 1") {
		t.Fatalf("plain diff = %q, %v", plain, err)
	}
	ignored, err := DiffTexts(ctx, old, next, WhitespaceIgnore)
	if err != nil || len(ignored) != 0 {
		t.Fatalf("ignored diff = %q, %v", ignored, err)
	}
	trimmed, err := DiffTexts(ctx, "a  \nb\n", "a\nb\n", WhitespaceTrim)
	if err != nil || len(trimmed) != 0 {
		t.Fatalf("trimmed diff = %q, %v", trimmed, err)
	}
}
