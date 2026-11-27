package syntax

import (
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

func TestFor(t *testing.T) {
	tests := []struct {
		name, filename, text, want string
	}{
		{"extension", "main.ex", "", "Elixir"},
		{"upper case extension", "a/b/Main.HS", "", "Haskell"},
		{"windows path", `C:\work\build.bat`, "", "Batch"},
		{"filename", "CMakeLists.txt", "", "CMake"},
		{"filename in dir", "/repo/Jenkinsfile", "", "Groovy"},
		{"bazel build", "pkg/BUILD.bazel", "", "Starlark"},
		{"dotenv", ".env", "", "Dotenv"},
		{"dotenv suffix", ".env.local", "", "Dotenv"},
		{"dotenv extension", "prod.env", "", "Dotenv"},
		{"gitignore", ".gitignore", "", "Ignore"},
		{"commit message", ".git/COMMIT_EDITMSG", "", "Git Commit"},
		{"rebase todo", ".git/rebase-merge/git-rebase-todo", "", "Git Rebase Todo"},
		{"log", "app.log", "", "Log"},
		{"csv", "data.csv", "", "CSV"},
		{"lisp scheme", "x.scm", "", "Lisp"},
		{"shebang env", "tool", "#!/usr/bin/env perl\nprint 1", "Perl"},
		{"shebang versioned", "tool", "#!/usr/bin/julia1.10\n", "Julia"},
		{"shebang env split", "tool", "#!/usr/bin/env -S elixir --no-halt\n", "Elixir"},
		{"shebang env var", "tool", "#!/usr/bin/env FOO=1 tclsh\n", "Tcl"},
		{"mygo extension", "main.go", "", "Go"},
		{"mygo shebang", "tool", "#!/bin/bash\n", "Shell"},
		{"mygo name wins over shebang", "run.sh", "#!/usr/bin/perl\n", "Shell"},
		{"astro", "src/pages/index.astro", "", "Astro"},
		{"svelte module", "store.svelte.ts", "", "TypeScript"},
		{"tsx", "App.tsx", "", "TypeScript React"},
		{"jsx", "App.jsx", "", "JavaScript"},
		{"gettext", "locales/de.po", "", "Gettext"},
		{"go mod", "backend/go.mod", "", "Go Module"},
		{"go sum", "go.sum", "", "Go Module"},
		{"go work", "go.work", "", "Go Module"},
		{"prettierrc", ".prettierrc", "", "JSON"},
		{"vscode settings", ".vscode/settings.json", "", "JSON"},
		{"dockerfile suffix", "Dockerfile.dev", "", "Dockerfile"},
		{"dockerfile", "docker/Dockerfile", "", "Dockerfile"},
		{"mdx", "docs/a.mdx", "", "MDX"},
		{"unknown", "notes.unknownext", "hello", ""},
		{"empty", "", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := For(tc.filename, tc.text)
			if name := nameOf(got); name != tc.want {
				t.Fatalf("For(%q) = %q, want %q", tc.filename, name, tc.want)
			}
		})
	}
}

func TestForName(t *testing.T) {
	tests := []struct{ filename, want string }{
		{"x.ex", "Elixir"}, {"x.go", "Go"}, {"x.unknownext", ""}, {"Dockerfile", "Dockerfile"},
	}
	for _, tc := range tests {
		if got := nameOf(ForName(tc.filename)); got != tc.want {
			t.Errorf("ForName(%q) = %q, want %q", tc.filename, got, tc.want)
		}
	}
}

func TestNamed(t *testing.T) {
	tests := []struct{ name, want string }{
		{"Elixir", "Elixir"}, {"elixir", "Elixir"}, {" F# ", "F#"}, {"fsharp", "F#"}, {"hs", "Haskell"},
		{"racket", "Lisp"}, {"Go", "Go"}, {"python", "Python"}, {"nonexistent", ""}, {"", ""},
	}
	for _, tc := range tests {
		if got := nameOf(Named(tc.name)); got != tc.want {
			t.Errorf("Named(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestRegistry(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range entries {
		l := e.lang
		if l.Name == "" || seen[l.Name] {
			t.Errorf("language name %q is empty or duplicated", l.Name)
		}
		seen[l.Name] = true
		if l.Tokenizer == nil {
			t.Errorf("%s has no tokenizer", l.Name)
		}
		for _, ext := range l.Extensions {
			if ext[0] != '.' {
				t.Errorf("%s extension %q has no dot", l.Name, ext)
			}
			if mine := editor.LanguageFor("x" + ext); mine != nil && !e.replaces {
				t.Errorf("%s extension %q is already MyGo's %s", l.Name, ext, mine.Name)
			}
		}
		for _, f := range l.Filenames {
			if mine := editor.LanguageFor(f); mine != nil && !e.replaces {
				t.Errorf("%s filename %q is already MyGo's %s", l.Name, f, mine.Name)
			}
		}
		for _, i := range l.Interpreters {
			if mine := editor.LanguageForText("x", "#!/usr/bin/"+i+"\n"); mine != nil && !e.replaces {
				t.Errorf("%s interpreter %q is already MyGo's %s", l.Name, i, mine.Name)
			}
		}
	}
}

func nameOf(l *editor.Language) string {
	if l == nil {
		return ""
	}
	return l.Name
}
