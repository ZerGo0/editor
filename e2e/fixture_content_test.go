//go:build e2e

package e2e

import (
	"fmt"
	"strings"
)

const (
	demoModules       = 20
	demoModuleFiles   = 5
	demoBigFileLines  = 5000
	demoLongDir1      = "internal/a-very-long-directory-name-that-keeps-going-and-going"
	demoLongDir2      = "another-extremely-long-sub-directory-name-for-overflow-checks"
	demoLongDir3      = "third-level-directory-with-a-name-longer-than-the-narrowest-sidebar"
	demoLongFile      = "file-with-a-name-that-is-also-much-longer-than-the-sidebar-width-for-sure.go"
	demoBigFile       = "generated/large_5000_lines.go"
	demoVersionFile   = "internal/version/version.go"
	demoChangelogDir  = "changelog"
	demoDuplicateName = "index.ts"
)

var demoModuleSubs = []string{"core", "api", "store", "view"}

type demoFile struct {
	Path string
	Text string
}

func demoModulePath(mod int, sub string, file int) string {
	return fmt.Sprintf("src/modules/mod%02d/%s/file%02d.go", mod, sub, file)
}

func demoModuleText(mod int, sub string, file int) string {
	return fmt.Sprintf("package %s\n\n// Handler%02d%02d serves requests for module %d.\nfunc Handler%02d%02d() string {\n\treturn %q\n}\n", sub, mod, file, mod, mod, file, sub)
}

func demoLanguageSamples() []demoFile {
	return []demoFile{
		{"samples/main.go", "package main\n\nimport \"fmt\"\n\n// TODO(alice): read the port from the environment\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"},
		{"samples/app.ts", "export function greet(name: string): string {\n  // FIXME: escape the name\n  return 'Hello ' + name\n}\n"},
		{"samples/view.tsx", "export const View = ({ title }: { title: string }) => <h1 className=\"title\">{title}</h1>\n// TODO: add a loading state\n"},
		{"samples/script.js", "const total = [1, 2, 3].reduce((a, b) => a + b, 0)\nconsole.log(total) // TODO remove\n"},
		{"samples/tool.py", "def total(values):\n    # TODO: handle empty input\n    return sum(values)\n"},
		{"samples/lib.rs", "pub fn add(a: i32, b: i32) -> i32 {\n    // FIXME: overflow\n    a + b\n}\n"},
		{"samples/Main.java", "public class Main {\n  public static void main(String[] args) {\n    System.out.println(\"hi\"); // TODO: logging\n  }\n}\n"},
		{"samples/util.c", "#include <stdio.h>\nint main(void) {\n  puts(\"hi\"); /* TODO: return code */\n  return 0;\n}\n"},
		{"samples/util.cpp", "#include <iostream>\nint main() { std::cout << \"hi\\n\"; }\n"},
		{"samples/util.h", "#pragma once\nint add(int a, int b);\n"},
		{"samples/Program.cs", "class Program { static void Main() { System.Console.WriteLine(\"hi\"); } }\n"},
		{"samples/app.rb", "class App\n  def run\n    puts 'hi' # TODO: logger\n  end\nend\n"},
		{"samples/index.php", "<?php\necho 'hi'; // TODO: escape\n"},
		{"samples/App.swift", "import Foundation\nprint(\"hi\") // TODO: localize\n"},
		{"samples/App.kt", "fun main() {\n    println(\"hi\") // TODO: args\n}\n"},
		{"samples/init.lua", "local M = {}\nfunction M.run() print('hi') end -- TODO\nreturn M\n"},
		{"samples/build.sh", "#!/usr/bin/env bash\nset -euo pipefail\necho building # TODO: parallelize\n"},
		{"samples/query.sql", "SELECT id, name FROM users WHERE active = 1 -- TODO: index\nORDER BY name;\n"},
		{"samples/page.html", "<!doctype html>\n<html><body><h1>Hello</h1><!-- TODO: footer --></body></html>\n"},
		{"samples/style.css", "body { margin: 0; font: 13px system-ui; } /* TODO: dark mode */\n"},
		{"samples/theme.scss", "$accent: #0078d4;\n.button { color: $accent; &:hover { opacity: .8; } }\n"},
		{"samples/data.json", "{\n  \"name\": \"demo\",\n  \"version\": \"1.0.0\"\n}\n"},
		{"samples/ci.yaml", "name: ci\non: [push]\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: go test ./...\n"},
		{"samples/Cargo.toml", "[package]\nname = \"demo\"\nversion = \"0.1.0\"\n"},
		{"samples/pom.xml", "<project>\n  <modelVersion>4.0.0</modelVersion>\n</project>\n"},
		{"samples/NOTES.md", "# Notes\n\n- [ ] TODO: write the docs\n- FIXME: the table below\n\n| a | b |\n|---|---|\n| 1 | 2 |\n"},
		{"samples/Dockerfile", "FROM golang:1.27\nWORKDIR /app\nCOPY . .\nRUN go build -o /bin/app .\n"},
		{"samples/Makefile", "build:\n\tgo build ./...\n\ntest:\n\tgo test ./...\n"},
		{"samples/api.proto", "syntax = \"proto3\";\nmessage User { string id = 1; string name = 2; }\n"},
		{"samples/schema.graphql", "type Query { user(id: ID!): User }\ntype User { id: ID! name: String! }\n"},
		{"samples/App.svelte", "<script>\n  let count = 0\n</script>\n<button on:click={() => count++}>{count}</button>\n"},
		{"samples/App.vue", "<template><div>{{ msg }}</div></template>\n<script>export default { data: () => ({ msg: 'hi' }) }</script>\n"},
		{"samples/main.dart", "void main() {\n  print('hi'); // TODO\n}\n"},
		{"samples/app.ex", "defmodule App do\n  def run, do: IO.puts(\"hi\")\nend\n"},
		{"samples/Main.hs", "main :: IO ()\nmain = putStrLn \"hi\" -- TODO\n"},
		{"samples/report.R", "x <- c(1, 2, 3)\nprint(mean(x)) # TODO\n"},
		{"samples/paper.tex", "\\documentclass{article}\n\\begin{document}\nHello % TODO\n\\end{document}\n"},
		{"samples/table.csv", "id,name,score\n1,Ada,10\n2,Grace,12\n"},
		{"samples/settings.ini", "[server]\nport = 8080\nhost = localhost\n"},
		{"samples/.env.example", "API_URL=http://localhost:8080\nAPI_TOKEN=\n"},
		{"samples/changes.diff", "--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-old\n+new\n"},
		{"samples/app.zsh", "#!/usr/bin/env zsh\nprint -r -- hi\n"},
	}
}

func demoNestedAndDuplicateFiles() []demoFile {
	long := demoLongDir1 + "/" + demoLongDir2 + "/" + demoLongDir3
	return []demoFile{
		{long + "/" + demoLongFile, "package long\n\n// TODO: rename this package, the path is absurdly long\nconst Name = \"long\"\n"},
		{long + "/short.go", "package long\n"},
		{demoLongDir1 + "/" + demoLongDir2 + "/notes-with-a-rather-long-name-about-nothing-in-particular.md", "# Long names\n\nThis file checks how rows truncate.\n"},
		{"web/app/" + demoDuplicateName, "export * from './routes'\n// TODO: lazy load routes\n"},
		{"web/components/button/" + demoDuplicateName, "export { Button } from './Button'\n"},
		{"web/lib/" + demoDuplicateName, "export const version = '1.0.0'\n"},
		{"web/app/routes.ts", "export const routes = ['/', '/settings']\n"},
		{"web/components/button/Button.tsx", "export const Button = () => <button>OK</button>\n"},
		{"internal/auth/utils.go", "package auth\n\nfunc Normalize(s string) string { return s }\n"},
		{"internal/billing/utils.go", "package billing\n\n// FIXME: rounding\nfunc Cents(f float64) int { return int(f * 100) }\n"},
		{"internal/render/utils.go", "package render\n\nfunc Clamp(v, lo, hi int) int { return min(max(v, lo), hi) }\n"},
		{"config.json", "{\n  \"mode\": \"production\"\n}\n"},
		{"docs/config.json", "{\n  \"mode\": \"docs\"\n}\n"},
		{"web/config.json", "{\n  \"mode\": \"web\"\n}\n"},
		{"README.md", "# Demo project\n\nA repository used to check the editor UI.\n\n- TODO: describe the layout\n- HACK: temporary notes\n"},
		{"docs/README.md", "# Docs\n\nSee the main README.\n"},
		{"web/README.md", "# Web\n\nFront end sources.\n"},
		{".gitignore", "build/\nnode_modules/\n*.log\n"},
		{"LICENSE", "MIT License\n\nCopyright (c) 2026 Demo\n"},
	}
}

func demoProblemFiles() []demoFile {
	return []demoFile{
		{"problems/unbalanced.go", "package problems\n\nfunc Open() {\n\tif true {\n\t\tprintln(\"never closed\")\n\t}\n"},
		{"problems/invalid.json", "{\n  \"name\": \"demo\",\n  \"items\": [1, 2,,]\n}\n"},
		{"problems/invalid.yaml", "root:\n  child: 1\n bad-indent: 2\n"},
		{"problems/mixed-eol.txt", "unix line\r\nwindows line\nunix again\r\n"},
	}
}

func demoModuleCatalog() []demoFile {
	var files []demoFile
	for mod := range demoModules {
		for _, sub := range demoModuleSubs {
			for file := range demoModuleFiles {
				files = append(files, demoFile{demoModulePath(mod, sub, file), demoModuleText(mod, sub, file)})
			}
		}
	}
	return files
}

func demoBigFileText() string {
	var b strings.Builder
	b.WriteString("package generated\n\n")
	for i := 0; i < demoBigFileLines-2; i++ {
		fmt.Fprintf(&b, "func Generated%04d() int { return %d } // TODO: line %d\n", i, i, i)
	}
	return b.String()
}

func demoConflictBase() []demoFile {
	return []demoFile{
		{"conflicts/settings.go", "package conflicts\n\nconst (\n\tTimeout = 30\n\tRetries = 3\n)\n"},
		{"conflicts/layout.json", "{\n  \"sidebar\": 260,\n  \"panel\": 200\n}\n"},
		{"conflicts/notes.md", "# Notes\n\nShared line.\n"},
	}
}

func demoConflictMain() []demoFile {
	return []demoFile{
		{"conflicts/settings.go", "package conflicts\n\nconst (\n\tTimeout = 60\n\tRetries = 5\n)\n"},
		{"conflicts/layout.json", "{\n  \"sidebar\": 300,\n  \"panel\": 200\n}\n"},
	}
}

func demoConflictTopic() []demoFile {
	return []demoFile{
		{"conflicts/settings.go", "package conflicts\n\nconst (\n\tTimeout = 45\n\tRetries = 3\n)\n"},
		{"conflicts/layout.json", "{\n  \"sidebar\": 240,\n  \"panel\": 200\n}\n"},
	}
}
