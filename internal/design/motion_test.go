package design

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

var motionCalls = map[string]bool{
	"Animate": true, "AnimateColor": true, "AnimatePose": true, "AnimateTo": true, "AnimateWith": true,
	"Keyframes": true, "Spring": true, "Transition": true,
}

func TestNoMotion(t *testing.T) {
	fset := token.NewFileSet()
	dirs := append([]string{"internal/design"}, ruleDirs...)
	for _, dir := range dirs {
		err := filepath.WalkDir(filepath.Join(repoRoot, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(f, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok && motionCalls[sel.Sel.Name] {
						t.Errorf("%s: %s animates; the app has no motion, set the target value directly", fset.Position(call.Pos()), sel.Sel.Name)
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
