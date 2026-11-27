package session

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLoadMissingFile(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "none.json"))
	if err != nil {
		t.Fatal(err)
	}
	if s.State.Projects == nil || len(s.State.Recent) != 0 || len(s.State.Open) != 0 {
		t.Fatalf("state %+v", s.State)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.json")
	if err := os.WriteFile(p, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(p)
	if err == nil {
		t.Fatal("expected error")
	}
	if s == nil || s.State.Projects == nil {
		t.Fatal("store should still be usable")
	}
}

func TestLoadNullProjects(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.json")
	if err := os.WriteFile(p, []byte(`{"recent":["/a"],"open":null,"projects":null}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if s.State.Projects == nil || !slices.Equal(s.State.Recent, []string{"/a"}) {
		t.Fatalf("state %+v", s.State)
	}
}

func TestCommandHistoryRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.json")
	s, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	s.State.Commands = []string{"workbench.action.files.save", "editor.action.formatDocument"}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.State.Commands, s.State.Commands) {
		t.Fatalf("commands %v", got.State.Commands)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.json")
	s, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	s.State.Recent = []string{"/a", "/b"}
	s.State.Open = []string{"/a"}
	s.State.Projects["/a"] = Project{
		SidebarWidth:  240,
		SidebarHidden: true,
		Expanded:      []string{"/a/src"},
		ActiveGroup:   1,
		Layout: &Node{
			Vertical: true,
			Percent:  40,
			First:    &Node{Group: &Group{Tabs: []Tab{{Path: "/a/x.go", Preview: true, Line: 3, Col: 4, ScrollY: 12.5}}, Active: 0}},
			Second:   &Node{Group: &Group{Tabs: []Tab{{Path: "/a/y.go"}}, Active: 0}},
		},
	}
	s.State.Projects["/b"] = Project{SidebarWidth: 100}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	pa := got.State.Projects["/a"]
	if !slices.Equal(got.State.Recent, []string{"/a", "/b"}) || !slices.Equal(got.State.Open, []string{"/a"}) {
		t.Fatalf("state %+v", got.State)
	}
	if pa.SidebarWidth != 240 || !pa.SidebarHidden || pa.ActiveGroup != 1 || !slices.Equal(pa.Expanded, []string{"/a/src"}) {
		t.Fatalf("project %+v", pa)
	}
	first := pa.Layout.First.Group.Tabs[0]
	if first != (Tab{Path: "/a/x.go", Preview: true, Line: 3, Col: 4, ScrollY: 12.5}) {
		t.Fatalf("tab %+v", first)
	}
	if !pa.Layout.Vertical || pa.Layout.Percent != 40 || pa.Layout.Second.Group.Tabs[0].Path != "/a/y.go" {
		t.Fatalf("layout %+v", pa.Layout)
	}
	if got.State.Projects["/b"].SidebarWidth != 100 {
		t.Fatal("project b lost")
	}
}

func TestSavePrunesProjects(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.json")
	s, _ := Load(p)
	s.State.Recent = []string{"/recent"}
	s.State.Open = []string{"/open"}
	for _, r := range []string{"/recent", "/open", "/stale"} {
		s.State.Projects[r] = Project{SidebarWidth: 1}
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	got, _ := Load(p)
	if len(got.State.Projects) != 2 {
		t.Fatalf("projects %v", got.State.Projects)
	}
	if _, ok := got.State.Projects["/stale"]; ok {
		t.Fatal("stale project kept")
	}
	if _, ok := s.State.Projects["/stale"]; ok {
		t.Fatal("in-memory stale project kept")
	}
}

func TestSaveFilePermission(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.json")
	s, _ := Load(p)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", info.Mode().Perm())
	}
}

func TestAddRecent(t *testing.T) {
	s, _ := Load(filepath.Join(t.TempDir(), "s.json"))
	s.AddRecent("/a")
	s.AddRecent("/b")
	s.AddRecent("/c")
	if !slices.Equal(s.State.Recent, []string{"/c", "/b", "/a"}) {
		t.Fatalf("recent %v", s.State.Recent)
	}
	s.AddRecent("/a")
	if !slices.Equal(s.State.Recent, []string{"/a", "/c", "/b"}) {
		t.Fatalf("recent %v", s.State.Recent)
	}
	s.AddRecent("/a")
	if !slices.Equal(s.State.Recent, []string{"/a", "/c", "/b"}) {
		t.Fatalf("recent %v", s.State.Recent)
	}
}

func TestAddRecentCaps(t *testing.T) {
	s, _ := Load(filepath.Join(t.TempDir(), "s.json"))
	for i := 0; i < maxRecent+5; i++ {
		s.AddRecent(fmt.Sprintf("/p%d", i))
	}
	if len(s.State.Recent) != maxRecent {
		t.Fatalf("len %d", len(s.State.Recent))
	}
	if s.State.Recent[0] != fmt.Sprintf("/p%d", maxRecent+4) || s.State.Recent[maxRecent-1] != "/p5" {
		t.Fatalf("recent %v", s.State.Recent)
	}
}

func TestRemoveRecent(t *testing.T) {
	s, _ := Load(filepath.Join(t.TempDir(), "s.json"))
	s.State.Recent = []string{"/a", "/b", "/c"}
	s.RemoveRecent("/b")
	if !slices.Equal(s.State.Recent, []string{"/a", "/c"}) {
		t.Fatalf("recent %v", s.State.Recent)
	}
	s.RemoveRecent("/missing")
	if len(s.State.Recent) != 2 {
		t.Fatalf("recent %v", s.State.Recent)
	}
}

func TestRecentFilesRoundTripNewestFirst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	s, _ := Load(path)
	for i := range maxRecentFiles + 1 {
		s.AddRecentFile(fmt.Sprintf("/p/f%d", i))
	}
	s.AddRecentFile("/p/f1")
	s.RemoveRecentFile("/p/f2")
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	files := got.State.Files
	if len(files) != maxRecentFiles-1 || files[0] != "/p/f1" || files[1] != fmt.Sprintf("/p/f%d", maxRecentFiles) || slices.Contains(files, "/p/f2") || slices.Contains(files, "/p/f0") {
		t.Fatalf("files = %v", files)
	}
}
