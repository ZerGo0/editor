package keymap_test

import (
	"testing"

	"github.com/ZerGo0/editor/internal/gitui"
	"github.com/ZerGo0/editor/internal/keymap"
)

func TestDefaultsBindGitActionKeys(t *testing.T) {
	k := keymap.New(keymap.Defaults(), nil)
	for _, a := range gitui.Actions {
		if a.Key == "" {
			continue
		}
		chord, err := keymap.ParseChord(a.Key)
		if err != nil {
			t.Fatalf("%s: %v", a.ID, err)
		}
		if b, ok := k.Lookup(chord); !ok || b.Command != a.ID {
			t.Errorf("%s runs %q, want %s", a.Key, b.Command, a.ID)
		}
	}
}

func TestDefaultsBindPreviewKeys(t *testing.T) {
	k := keymap.New(keymap.Defaults(), nil)
	for key, command := range map[string]string{
		"cmd+shift+v": "markdown.showPreview",
		"cmd+k v":     "markdown.showPreviewToSide",
	} {
		chord, err := keymap.ParseChord(key)
		if err != nil {
			t.Fatal(err)
		}
		if b, ok := k.Lookup(chord); !ok || b.Command != command {
			t.Errorf("%s runs %q, want %s", key, b.Command, command)
		}
	}
}
