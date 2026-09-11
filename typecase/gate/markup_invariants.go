package gate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

const maxInvariantTodos = 0

func checkInvariantGate(ref, widget string, catalog []CatalogEntry, widgetDir string) error {
	name := strings.TrimPrefix(ref, "gate:")
	spec, ok := invGateChecks[name]
	if !ok {
		return nil
	}
	html := widgetHTML(catalog, widget)
	return spec.fn(widget, html, widgetDir)
}

func checkStatValueIsString(widgetDir string) error {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, widgetDir, func(fi os.FileInfo) bool {
		return strings.HasSuffix(fi.Name(), ".go") && !strings.HasSuffix(fi.Name(), "_test.go") && fi.Name() != "doc.go"
	}, 0)
	if err != nil {
		return err
	}
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			var bad bool
			ast.Inspect(f, func(n ast.Node) bool {
				ts, ok := n.(*ast.TypeSpec)
				if !ok || ts.Name.Name != "Stat" {
					return true
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					return true
				}
				for _, field := range st.Fields.List {
					for _, name := range field.Names {
						if name.Name != "Value" {
							continue
						}
						ident, ok := field.Type.(*ast.Ident)
						if !ok || ident.Name != "string" {
							bad = true
							return false
						}
					}
				}
				return true
			})
			if bad {
				return fmt.Errorf("tc-stat: Value must be a pre-formatted string")
			}
		}
	}
	return nil
}
