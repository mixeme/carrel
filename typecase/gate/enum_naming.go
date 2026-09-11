package gate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

func checkConstAndEnumValues(widgetDir string) ([]string, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, widgetDir, func(fi os.FileInfo) bool {
		name := fi.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}, 0)
	if err != nil {
		return nil, err
	}
	var hits []string
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				vs, ok := n.(*ast.ValueSpec)
				if !ok {
					return true
				}
				for _, name := range vs.Names {
					if containsDomainWord(name.Name) {
						hits = append(hits, name.Name)
					}
				}
				for _, val := range vs.Values {
					if lit, ok := val.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						v := strings.Trim(lit.Value, `"`)
						if containsDomainWord(v) {
							hits = append(hits, v)
						}
					}
				}
				return true
			})
		}
	}
	return hits, nil
}

func checkDeclaredStates(widgetDir string) ([]string, error) {
	widgets, err := LoadWidgets(widgetDir)
	if err != nil {
		return nil, err
	}
	var hits []string
	for name, spec := range widgets {
		for _, st := range spec.States {
			if st == "default" {
				continue
			}
			if containsDomainWord(st) {
				hits = append(hits, name+"/"+st)
			}
		}
		text := spec.Sections["Состояния"]
		for _, part := range strings.Split(text, "|") {
			part = strings.TrimSpace(part)
			if i := strings.Index(part, " "); i > 0 {
				part = part[:i]
			}
			part = strings.TrimRight(part, ".,;")
			if part == "" || strings.HasPrefix(part, "по ") || strings.HasPrefix(part, "участников:") {
				continue
			}
			slug := slugState(part)
			if containsDomainWord(slug) {
				hits = append(hits, name+":state:"+slug)
			}
		}
	}
	return hits, nil
}

func checkRoleNaming(widgetDir string) error {
	constHits, err := checkConstAndEnumValues(widgetDir)
	if err != nil {
		return err
	}
	stateHits, err := checkDeclaredStates(widgetDir)
	if err != nil {
		return err
	}
	var all []string
	all = append(all, constHits...)
	all = append(all, stateHits...)
	if len(all) > 0 {
		return fmt.Errorf("domain-naming: %s", strings.Join(all, ", "))
	}
	return nil
}
