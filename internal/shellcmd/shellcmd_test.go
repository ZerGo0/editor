package shellcmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupHome(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func writeForeign(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("#!/bin/sh\necho foreign\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if p != filepath.Join(home, ".local", "bin", Name) {
		t.Fatalf("path %q", p)
	}
}

func TestInstall(t *testing.T) {
	want := setupHome(t)
	if Installed() {
		t.Fatal("installed before install")
	}
	got, err := Install("/Applications/Editor.app/Contents/MacOS/editor")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("path %q", got)
	}
	info, err := os.Stat(got)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("not executable: %v", info.Mode())
	}
	data, _ := os.ReadFile(got)
	s := string(data)
	if !strings.HasPrefix(s, "#!/bin/sh\n") || !strings.Contains(s, marker) {
		t.Fatalf("script %q", s)
	}
	if !strings.Contains(s, "'/Applications/Editor.app/Contents/MacOS/editor'") {
		t.Fatalf("script %q", s)
	}
	if !Installed() {
		t.Fatal("Installed false")
	}
}

func TestInstallQuoting(t *testing.T) {
	tests := []struct{ in, want string }{
		{"/a b/editor", "'/a b/editor'"},
		{"/it's/editor", `'/it'\''s/editor'`},
		{"/$x/`y`", "'/$x/`y`'"},
	}
	for _, tt := range tests {
		setupHome(t)
		p, err := Install(tt.in)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(p)
		if !strings.Contains(string(data), "nohup "+tt.want+" \"$@\"") {
			t.Errorf("in %q: script %q", tt.in, data)
		}
	}
}

func TestInstallOverwritesOwned(t *testing.T) {
	setupHome(t)
	if _, err := Install("/old/editor"); err != nil {
		t.Fatal(err)
	}
	p, err := Install("/new/editor")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	if strings.Contains(string(data), "/old/") || !strings.Contains(string(data), "'/new/editor'") {
		t.Fatalf("script %q", data)
	}
}

func TestUninstall(t *testing.T) {
	setupHome(t)
	if _, err := Install("/x/editor"); err != nil {
		t.Fatal(err)
	}
	p, err := Uninstall()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file remains: %v", err)
	}
	if Installed() {
		t.Fatal("still installed")
	}
}

func TestUninstallNothingInstalled(t *testing.T) {
	want := setupHome(t)
	got, err := Uninstall()
	if err != nil || got != want {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestInstallRefusesForeign(t *testing.T) {
	p := setupHome(t)
	writeForeign(t, p)
	before, _ := os.ReadFile(p)
	got, err := Install("/x/editor")
	if !errors.Is(err, ErrForeign) || got != p {
		t.Fatalf("got %q err %v", got, err)
	}
	after, _ := os.ReadFile(p)
	if string(before) != string(after) {
		t.Fatal("foreign file changed")
	}
	if Installed() {
		t.Fatal("foreign file reported as installed")
	}
}

func TestUninstallRefusesForeign(t *testing.T) {
	p := setupHome(t)
	writeForeign(t, p)
	before, _ := os.ReadFile(p)
	_, err := Uninstall()
	if !errors.Is(err, ErrForeign) {
		t.Fatalf("err %v", err)
	}
	after, err := os.ReadFile(p)
	if err != nil || string(before) != string(after) {
		t.Fatal("foreign file changed")
	}
}
