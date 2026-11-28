package design

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	repoRoot       = "../.."
	allowlistPath  = "testdata/rules_allowlist.txt"
	updateEnv      = "DESIGN_RULES_UPDATE"
	allowDirective = "//design:allow"
	uiPackage      = "ui"
)

var ruleDirs = []string{
	"internal/app", "internal/workspace", "internal/gitui", "internal/prui", "internal/issueui", "internal/search/finder",
	"internal/viewer", "internal/insights", "internal/palette", "internal/prefs", "internal/largefile", "internal/history",
	"internal/findwidget", "internal/git/diffmode", "internal/reviewui",
}

var visualSetters = map[string]bool{
	"Padding": true, "Margin": true, "Gap": true, "FontSize": true, "FontWeight": true, "Radius": true,
	"Height": true, "Width": true, "MinWidth": true, "MaxWidth": true, "MinHeight": true, "MaxHeight": true,
	"Size": true, "Border": true, "BorderWidth": true, "LineHeight": true, "LetterSpacing": true, "Opacity": true,
	"Shadow": true, "Top": true, "Left": true, "Right": true, "Bottom": true, "ZIndex": true,
	"FocusRingWidth": true, "FocusRingOffset": true, "Indent": true,
}

var colorConstructors = map[string]bool{"RGB": true, "RGBA": true, "Hex": true}

var rawWidgets = map[string]string{
	"Button": "action.Button", "ButtonBase": "action.Button or action.IconButton", "TextInput": "field.TextInput",
	"TextInputBase": "field.TextInput", "TextArea": "field.TextArea", "TextAreaBase": "field.TextArea",
	"Checkbox": "action.Checkbox", "CheckboxBase": "action.Checkbox", "Switch": "action.Switch", "SwitchBase": "action.Switch",
	"Select": "action.Select", "SelectBase": "action.Select", "Tabs": "chrome.TabStrip", "TabsBase": "chrome.TabStrip",
	"Toggle": "action.ToggleIconButton", "ToggleBase": "action.ToggleIconButton", "Progress": "feedback.Progress",
	"Spinner": "feedback.Spinner",
}

type violation struct {
	file, rule, why string
	line, col       int
}

func (v violation) String() string {
	return fmt.Sprintf("%s:%d:%d: %s: %s", v.file, v.line, v.col, v.rule, v.why)
}

type fileScan struct {
	fset   *token.FileSet
	path   string
	consts map[string]bool
	allow  map[int]string
	out    []violation
}

func TestDesignRules(t *testing.T) {
	found := scanRules(t)
	counts := map[string]int{}
	byKey := map[string][]violation{}
	for _, v := range found {
		k := v.file + " " + v.rule
		counts[k]++
		byKey[k] = append(byKey[k], v)
	}
	allowed, err := readAllowlist()
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if os.Getenv(updateEnv) != "" {
		updateAllowlist(t, allowed, counts, err == nil)
		return
	}
	keys := slices.Sorted(func(yield func(string) bool) {
		for k := range counts {
			if !yield(k) {
				return
			}
		}
	})
	for _, k := range keys {
		if counts[k] <= allowed[k] {
			continue
		}
		for _, v := range byKey[k] {
			t.Error(v)
		}
		t.Errorf("%s: %d design rule violations, the allowlist permits %d. Use internal/design components and tokens; for a rare exception add \"%s <reason>\" on the line", k, counts[k], allowed[k], allowDirective)
	}
	for k, n := range allowed {
		if counts[k] < n {
			t.Errorf("%s: allowlist permits %d, found %d. Shrink it: %s=1 go test ./internal/design -run TestDesignRules", k, n, counts[k], updateEnv)
		}
	}
}

func scanRules(t *testing.T) []violation {
	var out []violation
	for _, dir := range ruleDirs {
		root := filepath.Join(repoRoot, dir)
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}
		pkgConsts := map[string]map[string]bool{}
		var files []string
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		fset := token.NewFileSet()
		parsed := map[string]*ast.File{}
		for _, path := range files {
			f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			if ast.IsGenerated(f) {
				continue
			}
			parsed[path] = f
			dirConsts := pkgConsts[filepath.Dir(path)]
			if dirConsts == nil {
				dirConsts = map[string]bool{}
				pkgConsts[filepath.Dir(path)] = dirConsts
			}
			collectNumericConsts(f, dirConsts)
		}
		for path, f := range parsed {
			rel, _ := filepath.Rel(repoRoot, path)
			s := &fileScan{fset: fset, path: filepath.ToSlash(rel), consts: pkgConsts[filepath.Dir(path)], allow: directives(fset, f)}
			s.scan(f)
			out = append(out, s.out...)
		}
	}
	return out
}

func collectNumericConsts(f *ast.File, into map[string]bool) {
	ast.Inspect(f, func(n ast.Node) bool {
		decl, ok := n.(*ast.GenDecl)
		if !ok || decl.Tok != token.CONST {
			return true
		}
		for _, spec := range decl.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				if i < len(vs.Values) && hasNumber(vs.Values[i]) {
					into[name.Name] = true
				}
			}
		}
		return true
	})
}

func directives(fset *token.FileSet, f *ast.File) map[int]string {
	out := map[int]string{}
	for _, group := range f.Comments {
		for _, c := range group.List {
			if !strings.HasPrefix(c.Text, allowDirective) {
				continue
			}
			line := fset.Position(c.Slash).Line
			reason := strings.TrimSpace(strings.TrimPrefix(c.Text, allowDirective))
			out[line], out[line+1] = reason, reason
		}
	}
	return out
}

func (s *fileScan) report(n ast.Node, rule, why string) {
	pos := s.fset.Position(n.Pos())
	if reason, ok := s.allow[pos.Line]; ok {
		if reason != "" {
			return
		}
		why = allowDirective + " needs a reason; " + why
	}
	s.out = append(s.out, violation{file: s.path, rule: rule, why: why, line: pos.Line, col: pos.Column})
}

func (s *fileScan) scan(f *ast.File) {
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CompositeLit:
			if isUI(n.Type, "Color") {
				s.report(n, "color-literal", "ui.Color literal: use a palette role from token.Of(c)")
			}
		case *ast.CallExpr:
			s.call(n)
		}
		return true
	})
}

func (s *fileScan) call(n *ast.CallExpr) {
	sel, ok := n.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	name := sel.Sel.Name
	if isUI(sel, name) {
		switch {
		case colorConstructors[name]:
			s.report(n, "color-literal", "ui."+name+" builds a color: use a palette role from token.Of(c)")
		case rawWidgets[name] != "":
			s.report(n, "raw-widget", "ui."+name+" draws its own look: use "+rawWidgets[name])
		}
		return
	}
	switch {
	case name == "Theme" && len(n.Args) == 0:
		s.report(n, "theme-access", "reads the MyGo theme: use token.Of(c) roles")
	case name == "Font" && len(n.Args) == 1 && isString(n.Args[0]):
		s.report(n, "raw-font", "font family literal: use a token.TextStyle such as token.TextMono")
	case visualSetters[name]:
		for _, arg := range n.Args {
			if lit := s.rawValue(arg); lit != "" {
				s.report(arg, "raw-number", lit+" in "+name+": use a token from internal/design/token, or a design component")
				return
			}
		}
	}
}

func (s *fileScan) rawValue(arg ast.Expr) string {
	found := ""
	ast.Inspect(arg, func(n ast.Node) bool {
		if found != "" {
			return false
		}
		switch n := n.(type) {
		case *ast.BasicLit:
			if (n.Kind == token.INT || n.Kind == token.FLOAT) && !isZero(n.Value) {
				found = n.Value
			}
		case *ast.Ident:
			if s.consts[n.Name] {
				found = "constant " + n.Name
			}
		case *ast.SelectorExpr:
			return false
		}
		return true
	})
	return found
}

func hasNumber(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && (lit.Kind == token.INT || lit.Kind == token.FLOAT) {
			found = true
		}
		return !found
	})
	return found
}

func isZero(v string) bool {
	f, err := strconv.ParseFloat(v, 64)
	return err == nil && f == 0
}

func isString(e ast.Expr) bool {
	lit, ok := e.(*ast.BasicLit)
	return ok && lit.Kind == token.STRING
}

func isUI(e ast.Expr, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == uiPackage
}

func readAllowlist() (map[string]int, error) {
	out := map[string]int{}
	f, err := os.Open(allowlistPath)
	if err != nil {
		return out, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 3 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		n, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("%s: bad count %q", allowlistPath, fields[2])
		}
		out[fields[0]+" "+fields[1]] = n
	}
	return out, sc.Err()
}

func updateAllowlist(t *testing.T, allowed, counts map[string]int, existed bool) {
	if existed {
		for k, n := range counts {
			if n > allowed[k] {
				t.Fatalf("%s: %d violations, allowlist permits %d. The allowlist only shrinks: fix the new violations", k, n, allowed[k])
			}
		}
	}
	keys := slices.Sorted(func(yield func(string) bool) {
		for k := range counts {
			if !yield(k) {
				return
			}
		}
	})
	var b strings.Builder
	b.WriteString("# Design rule violations left from before the design system. Generated by TestDesignRules; it may only shrink.\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "%s %d\n", k, counts[k])
	}
	if err := os.MkdirAll(filepath.Dir(allowlistPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(allowlistPath, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
