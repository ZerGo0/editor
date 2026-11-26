package command_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/e2e/qa"
	"github.com/ZerGo0/editor/internal/gitui"
	"github.com/ZerGo0/editor/internal/history"
	"github.com/ZerGo0/editor/internal/issueui"
	"github.com/ZerGo0/editor/internal/prui"
	"github.com/ZerGo0/editor/internal/shellcmd"
)

const (
	appDir          = "../app"
	commandCallName = "add"
	menuCallName    = "item"
	addArgsMin      = 3
	menuArgs        = 2
	titleArg        = 2
	idArg           = 0
	commandType     = "Command"
	idField         = "ID"
	titleField      = "Title"
	shellNameExpr   = "shellcmd.Name"
)

var appSources = []string{"commands.go", "keymapimport.go", "menu.go"}

var strictRules = map[string]bool{
	qa.RuleDoubleSpace:   true,
	qa.RuleEdgeSpace:     true,
	qa.RuleTrailingPunct: true,
	qa.RuleTitleCase:     true,
}

type entry struct {
	ID     string
	Title  string
	Source string
	Menu   bool
}

type resolver struct {
	consts map[string]string
}

func (r resolver) str(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(v.Value)
		return s, err == nil
	case *ast.Ident:
		s, ok := r.consts[v.Name]
		return s, ok
	case *ast.SelectorExpr:
		if x, ok := v.X.(*ast.Ident); ok && x.Name+"."+v.Sel.Name == shellNameExpr {
			return shellcmd.Name, true
		}
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return "", false
		}
		l, lok := r.str(v.X)
		rr, rok := r.str(v.Y)
		return l + rr, lok && rok
	case *ast.ParenExpr:
		return r.str(v.X)
	}
	return "", false
}

func parseApp(t *testing.T) []*ast.File {
	t.Helper()
	fset := token.NewFileSet()
	var files []*ast.File
	for _, name := range appSources {
		f, err := parser.ParseFile(fset, filepath.Join(appDir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, f)
	}
	return files
}

func constTable(files []*ast.File) resolver {
	r := resolver{consts: map[string]string{}}
	for pass := 0; pass < 2; pass++ {
		for _, f := range files {
			for _, decl := range f.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.CONST {
					continue
				}
				for _, spec := range gen.Specs {
					vs := spec.(*ast.ValueSpec)
					for i, name := range vs.Names {
						if i < len(vs.Values) {
							if s, ok := r.str(vs.Values[i]); ok {
								r.consts[name.Name] = s
							}
						}
					}
				}
			}
		}
	}
	return r
}

func appEntries(t *testing.T) (commands, menus []entry) {
	t.Helper()
	files := parseApp(t)
	r := constTable(files)
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.CallExpr:
				switch fn := v.Fun.(type) {
				case *ast.Ident:
					if fn.Name == commandCallName && len(v.Args) >= addArgsMin {
						id, idOK := r.str(v.Args[idArg])
						title, titleOK := r.str(v.Args[titleArg])
						if idOK && titleOK {
							commands = append(commands, entry{ID: id, Title: title, Source: "app/commands.go"})
						}
					}
				case *ast.SelectorExpr:
					if fn.Sel.Name == menuCallName && len(v.Args) == menuArgs {
						id, idOK := r.str(v.Args[0])
						label, labelOK := r.str(v.Args[1])
						if idOK && labelOK {
							menus = append(menus, entry{ID: id, Title: label, Source: "app/menu.go", Menu: true})
						}
					}
				}
			case *ast.CompositeLit:
				sel, ok := v.Type.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != commandType {
					return true
				}
				var e entry
				for _, el := range v.Elts {
					kv, ok := el.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					key, _ := kv.Key.(*ast.Ident)
					if key == nil {
						continue
					}
					switch key.Name {
					case idField:
						e.ID, _ = r.str(kv.Value)
					case titleField:
						e.Title, _ = r.str(kv.Value)
					}
				}
				if e.ID != "" && e.Title != "" {
					e.Source = "app/keymapimport.go"
					commands = append(commands, e)
				}
			}
			return true
		})
	}
	return commands, menus
}

func tableEntries() []entry {
	var out []entry
	for _, a := range gitui.Actions {
		out = append(out, entry{ID: a.ID, Title: a.Title, Source: "gitui/actions.go"})
	}
	for _, a := range history.Actions {
		out = append(out, entry{ID: a.ID, Title: a.Title, Source: "history/ui.go"})
	}
	for _, a := range prui.Actions {
		out = append(out, entry{ID: a.ID, Title: a.Title, Source: "prui/actions.go"})
	}
	for _, a := range issueui.Actions {
		out = append(out, entry{ID: a.ID, Title: a.Title, Source: "issueui/actions.go"})
	}
	return out
}

func titleOf(e entry, commands map[string]string) string {
	if e.Menu && e.Title == "" {
		return commands[e.ID]
	}
	return e.Title
}

func checkEntry(e entry, title string) []qa.Issue {
	opens, classified := qa.OpensInput[e.ID]
	issues := qa.CheckTitle(title, opens, classified)
	issues = append(issues, qa.CheckVSCodeWords(title)...)
	if issue := qa.CompareVSCode(e.ID, title); issue != nil {
		issues = append(issues, *issue)
	}
	return issues
}

func TestCommandAndMenuWording(t *testing.T) {
	commands, menus := appEntries(t)
	commands = append(commands, tableEntries()...)
	if len(commands) < 50 || len(menus) < 20 {
		t.Fatalf("parsed %d commands and %d menu items: the source layout changed, update the collector", len(commands), len(menus))
	}
	titles := map[string]string{}
	for _, c := range commands {
		titles[c.ID] = c.Title
	}
	var strict, advisory []string
	seen := map[string]string{}
	check := func(e entry) {
		title := e.Title
		kind := "command"
		if e.Menu {
			kind = "menu"
			if title == "" {
				title = titles[e.ID]
			}
		}
		if title == "" {
			return
		}
		key := kind + "|" + e.ID + "|" + title
		if seen[key] != "" {
			return
		}
		seen[key] = key
		for _, issue := range checkEntry(e, title) {
			line := kind + " " + e.ID + " (" + e.Source + "): " + issue.String()
			if strictRules[issue.Rule] {
				strict = append(strict, line)
			} else {
				advisory = append(advisory, line)
			}
		}
	}
	for _, c := range commands {
		check(c)
	}
	for _, m := range menus {
		check(m)
	}
	byLabel := map[string][]string{}
	for _, c := range commands {
		byLabel[c.Title] = append(byLabel[c.Title], c.ID)
	}
	for title, ids := range byLabel {
		if len(ids) > 1 {
			slices.Sort(ids)
			advisory = append(advisory, "duplicate title "+strconv.Quote(title)+" for "+strings.Join(ids, ", "))
		}
	}
	slices.Sort(advisory)
	for _, line := range advisory {
		t.Log("not enforced yet: " + line)
	}
	for _, line := range strict {
		t.Error(line)
	}
}
