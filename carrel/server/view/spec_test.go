package view

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// specFile is the seam itself: C-6, C-8 and C-9 of carrel-common.md.
const specFile = "../../../docs/spec/carrel-common.md"

// goBlock matches a fenced Go block. The working copy may have CRLF
// endings (git converts them on checkout on Windows), so the fence is
// matched either way.
var goBlock = regexp.MustCompile("(?s)```go\r?\n(.*?)```")

// parseSpec reads every fenced Go block out of the spec and parses them as
// one file. The blocks are valid Go, so the declarations can be compared
// with this package's own rather than eyeballed.
func parseSpec(t *testing.T) *ast.File {
	t.Helper()
	src, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	var b strings.Builder
	b.WriteString("package spec\n")
	blocks := goBlock.FindAllSubmatch(src, -1)
	if len(blocks) == 0 {
		t.Fatalf("no fenced Go blocks in %s", specFile)
	}
	for _, m := range blocks {
		b.Write(m[1])
		b.WriteString("\n")
	}
	f, err := parser.ParseFile(token.NewFileSet(), "spec.go", b.String(), 0)
	if err != nil {
		t.Fatalf("parse spec blocks as Go: %v", err)
	}
	return f
}

// parsePackage parses this package's own non-test sources as one file set.
func parsePackage(t *testing.T) []*ast.File {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, f)
	}
	return files
}

// shape renders a type declaration as a single comparable line: the field
// names and their types for a struct, the underlying type for a named
// string. Comments and formatting do not take part.
func shape(spec *ast.TypeSpec) string {
	st, ok := spec.Type.(*ast.StructType)
	if !ok {
		return types.ExprString(spec.Type)
	}
	var parts []string
	for _, f := range st.Fields.List {
		typ := types.ExprString(f.Type)
		if len(f.Names) == 0 {
			parts = append(parts, typ) // embedded
			continue
		}
		for _, n := range f.Names {
			parts = append(parts, n.Name+" "+typ)
		}
	}
	return "struct{" + strings.Join(parts, "; ") + "}"
}

func typeShapes(files ...*ast.File) map[string]string {
	out := map[string]string{}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSpec); ok {
				out[ts.Name.Name] = shape(ts)
			}
			return true
		})
	}
	return out
}

// TestTypesMatchSpec: every type in C-6, C-8 and C-9 is declared here with
// the same fields, in the same order, with the same types — and nothing
// beyond them. A field that exists on no frame gets drawn one day (C-6.1).
func TestTypesMatchSpec(t *testing.T) {
	want := typeShapes(parseSpec(t))
	got := typeShapes(parsePackage(t)...)

	for name, w := range want {
		g, ok := got[name]
		if !ok {
			t.Errorf("type %s is in the spec but not in the package", name)
			continue
		}
		if g != w {
			t.Errorf("type %s differs from the spec:\n  spec: %s\n  code: %s", name, w, g)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("type %s is in the package but not in the spec", name)
		}
	}
}

// constValues collects the members of every typed string const block as
// "Name = value", keyed by the type named on its first line.
func constValues(files ...*ast.File) map[string][]string {
	out := map[string][]string{}
	for _, f := range files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			typ := ""
			for _, s := range gd.Specs {
				vs, ok := s.(*ast.ValueSpec)
				if !ok || len(vs.Values) == 0 {
					continue
				}
				if id, ok := vs.Type.(*ast.Ident); ok {
					typ = id.Name
				}
				lit, ok := vs.Values[0].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING || typ == "" {
					continue
				}
				v, err := strconv.Unquote(lit.Value)
				if err != nil {
					continue
				}
				out[typ] = append(out[typ], vs.Names[0].Name+" = "+v)
			}
		}
	}
	return out
}

// TestEnumValuesMatchSpec: every C-8 enum carries the same members, named
// the same and in the same order, with the same values. The values cross
// the wire; the names are what the rest of the server will be written in.
func TestEnumValuesMatchSpec(t *testing.T) {
	want := constValues(parseSpec(t))
	got := constValues(parsePackage(t)...)
	if len(want) == 0 {
		t.Fatal("no typed string constants found in the spec")
	}
	for typ, w := range want {
		g, ok := got[typ]
		if !ok {
			t.Errorf("enum %s is in the spec but has no constants here", typ)
			continue
		}
		if fmt.Sprint(g) != fmt.Sprint(w) {
			t.Errorf("enum %s differs from the spec:\n  spec: %v\n  code: %v", typ, w, g)
		}
	}
	for typ := range got {
		if _, ok := want[typ]; !ok {
			t.Errorf("enum %s has constants here but is not in the spec", typ)
		}
	}
}

// actionName is how C-7.5 writes an action's id in backticks: lower-case
// words joined by dots. Field names (`Action.ID`, `Menu`) are capitalised.
var actionName = regexp.MustCompile("`([a-z][a-z0-9]*(?:[.][a-z0-9-]+)*)`")

// TestSpecActionIDsExist: every action id C-7.5 names is one the server
// gives — carried by a fixture, or written as a string in internal/web.
// The two sides come from different places: the spec's words, and what the
// fixtures and the code hold. link.revoke stood in C-7.5 long after the
// server named it publish.revoke; renames of ids (record.*, publish,
// attach.*, collection.*) turn this red when the spec is left behind.
func TestSpecActionIDsExist(t *testing.T) {
	src, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatal(err)
	}
	spec := strings.ReplaceAll(string(src), "\r\n", "\n")
	_, section, ok := strings.Cut(spec, "\n### C-7.5 ")
	if !ok {
		t.Fatal("no C-7.5 in the spec")
	}
	if end := strings.Index(section, "\n---"); end >= 0 {
		section = section[:end]
	}
	named := map[string]bool{}
	for _, m := range actionName.FindAllStringSubmatch(section, -1) {
		named[m[1]] = true
	}
	if len(named) < 5 {
		t.Fatalf("C-7.5 names %d action ids: the section was not read", len(named))
	}

	known := map[string]bool{}
	eachFixtureAction(t, func(_, _ string, a Action) { known[a.ID] = true })
	const web = "../internal/web"
	entries, err := os.ReadDir(web)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, web+"/"+name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if v, err := strconv.Unquote(lit.Value); err == nil {
					known[v] = true
				}
			}
			return true
		})
	}
	for id := range named {
		if !known[id] {
			t.Errorf("C-7.5 names the action %q, which no fixture carries and internal/web does not write", id)
		}
	}
}
