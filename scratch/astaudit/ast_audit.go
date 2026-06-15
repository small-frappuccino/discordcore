package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	fset := token.NewFileSet()
	var deviations []string

	err := filepath.Walk("./pkg", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		node, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil
		}

		ast.Inspect(node, func(n ast.Node) bool {
			// Look for if stmt checking `id == ""`
			if ifStmt, ok := n.(*ast.IfStmt); ok {
				if binOp, ok := ifStmt.Cond.(*ast.BinaryExpr); ok {
					if binOp.Op == token.EQL {
						var hasEmptyString bool
						var hasBotID bool

						// Check left and right sides
						for _, expr := range []ast.Expr{binOp.X, binOp.Y} {
							if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING && lit.Value == `""` {
								hasEmptyString = true
							} else if ident, ok := expr.(*ast.Ident); ok {
								name := strings.ToLower(ident.Name)
								if strings.Contains(name, "botid") || strings.Contains(name, "botinstanceid") || strings.Contains(name, "target") {
									hasBotID = true
								}
							} else if sel, ok := expr.(*ast.SelectorExpr); ok {
								name := strings.ToLower(sel.Sel.Name)
								if strings.Contains(name, "botid") || strings.Contains(name, "botinstanceid") {
									hasBotID = true
								}
							}
						}

						if hasEmptyString && hasBotID {
							pos := fset.Position(ifStmt.Pos())
							deviations = append(deviations, fmt.Sprintf("%s: found potential deviation checking bot ID against \"\": %s", pos.String(), renderNode(fset, ifStmt.Cond)))
						}
					}
				}
			}
			return true
		})
		return nil
	})

	if err != nil {
		panic(err)
	}

	for _, d := range deviations {
		fmt.Println(d)
	}
}

func renderNode(fset *token.FileSet, node ast.Node) string {
	return "" // Simplified for logging
}
