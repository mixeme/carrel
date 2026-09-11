package gate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strings"
)

var docInputLine = regexp.MustCompile(`^(\w+)\{\s*([^}]*?)\s*\}\.?$`)

func loadWidgetStructFields(widgetDir string) (map[string][]string, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, widgetDir, func(fi os.FileInfo) bool {
		name := fi.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") && name != "doc.go"
	}, 0)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]string)
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				ts, ok := n.(*ast.TypeSpec)
				if !ok {
					return true
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					return true
				}
				var names []string
				for _, field := range st.Fields.List {
					for _, name := range field.Names {
						if name.IsExported() {
							names = append(names, name.Name)
						}
					}
				}
				out[ts.Name.Name] = names
				return true
			})
		}
	}
	return out, nil
}

func parseDocInput(text string) (typeName string, fields []string, err error) {
	text = strings.TrimSpace(text)
	m := docInputLine.FindStringSubmatch(text)
	if m == nil {
		return "", nil, fmt.Errorf("cannot parse вход %q", text)
	}
	typeName = m[1]
	raw := strings.TrimSpace(m[2])
	if raw == "" {
		return typeName, nil, nil
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if i := strings.Index(part, " "); i > 0 {
			part = part[:i]
		}
		fields = append(fields, part)
	}
	return typeName, fields, nil
}

func fieldSetDiff(docFields, goFields []string) (missingInDoc, extraInDoc []string) {
	doc := make(map[string]bool, len(docFields))
	for _, f := range docFields {
		doc[f] = true
	}
	goF := make(map[string]bool, len(goFields))
	for _, f := range goFields {
		goF[f] = true
		if !doc[f] {
			missingInDoc = append(missingInDoc, f)
		}
	}
	for _, f := range docFields {
		if !goF[f] {
			extraInDoc = append(extraInDoc, f)
		}
	}
	sort.Strings(missingInDoc)
	sort.Strings(extraInDoc)
	return missingInDoc, extraInDoc
}

// CheckDocInput matches L-3.2 «Вход» field lists to exported struct fields.
func CheckDocInput(widgetDir string, widgets map[string]WidgetSpec) error {
	structFields, err := loadWidgetStructFields(widgetDir)
	if err != nil {
		return err
	}
	var problems []string
	for name, spec := range widgets {
		inputText, ok := spec.Sections["Вход"]
		if !ok {
			continue
		}
		typeName, docFields, err := parseDocInput(inputText)
		if err != nil {
			problems = append(problems, name+": "+err.Error())
			continue
		}
		if typeName != spec.GoType {
			problems = append(problems, fmt.Sprintf("%s: вход names %s, struct is %s", name, typeName, spec.GoType))
			continue
		}
		goFields, ok := structFields[spec.GoType]
		if !ok {
			problems = append(problems, name+": struct "+spec.GoType+" not found")
			continue
		}
		missing, extra := fieldSetDiff(docFields, goFields)
		if len(missing) > 0 {
			problems = append(problems, fmt.Sprintf("%s: вход missing %s", name, strings.Join(missing, ", ")))
		}
		if len(extra) > 0 {
			problems = append(problems, fmt.Sprintf("%s: вход extra %s", name, strings.Join(extra, ", ")))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("doc-format: %s", strings.Join(problems, "; "))
	}
	return nil
}
