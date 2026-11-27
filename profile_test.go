package main

import "testing"

func TestInstalledOnlyForBundlesInApplications(t *testing.T) {
	const home = "/Users/me"
	cases := []struct {
		exe  string
		want bool
	}{
		{"/Applications/Editor.app/Contents/MacOS/Editor", true},
		{"/Users/me/Applications/Editor.app/Contents/MacOS/Editor", true},
		{"/Users/me/code/editor/build/Editor.app/Contents/MacOS/Editor", false},
		{"/Users/me/code/editor/editor", false},
		{"/Applications/editor", false},
		{"/ApplicationsX/Editor.app/Contents/MacOS/Editor", false},
	}
	for _, c := range cases {
		if got := installed(c.exe, home); got != c.want {
			t.Errorf("installed(%q) = %v, want %v", c.exe, got, c.want)
		}
	}
}
