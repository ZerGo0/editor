package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ZerGo0/editor/internal/document"
	"github.com/ZerGo0/editor/internal/viewer"
)

func TestLoadFileChoosesViewer(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name     string
		data     string
		asText   bool
		kind     viewer.Kind
		editable bool
	}{
		{name: "shot.png", data: "\x89PNG\r\n\x1a\n", kind: viewer.Image},
		{name: "blob", data: "a\x00b", kind: viewer.Hex},
		{name: "blob", data: "a\x00b", asText: true, kind: viewer.None, editable: true},
		{name: "README.md", data: "# Title\n", kind: viewer.None, editable: true},
	}
	for _, tt := range tests {
		p := filepath.Join(dir, tt.name)
		if err := os.WriteFile(p, []byte(tt.data), 0o644); err != nil {
			t.Fatal(err)
		}
		d, kind, stamp, err := loadFile(p, tt.asText)
		if err != nil || kind != tt.kind || (d != nil && d.Editable()) != tt.editable {
			t.Fatalf("%s as text %v: %v %v %v", tt.name, tt.asText, d, kind, err)
		}
		if kind.ReplacesEditor() && stamp.size != int64(len(tt.data)) {
			t.Fatalf("%s: stamp %+v", tt.name, stamp)
		}
		if tt.asText && d.Kind != document.Text {
			t.Fatalf("%s: kind %v", tt.name, d.Kind)
		}
	}
	if _, _, _, err := loadFile(filepath.Join(dir, "missing"), false); err == nil {
		t.Fatal("missing file loaded")
	}
}
