package view

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// structs parses this package's own sources and returns every struct type
// declared in it, keyed by name. The schema is the contract (C-3.1), so it
// is checked from the declarations themselves rather than from a registry
// that would have to be kept in step with them by hand.
func structs(t *testing.T) map[string]*ast.StructType {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}
	out := map[string]*ast.StructType{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				ts, ok := n.(*ast.TypeSpec)
				if !ok {
					return true
				}
				if st, ok := ts.Type.(*ast.StructType); ok {
					out[ts.Name.Name] = st
				}
				return true
			})
		}
	}
	if len(out) == 0 {
		t.Fatal("no struct types found in package")
	}
	return out
}

// tagOf returns the json name of a field, and whether it carries a tag.
func tagOf(f *ast.Field) (string, bool) {
	if f.Tag == nil {
		return "", false
	}
	lit, err := strconv.Unquote(f.Tag.Value)
	if err != nil {
		return "", false
	}
	v, ok := reflect.StructTag(lit).Lookup("json")
	if !ok {
		return "", false
	}
	return strings.Split(v, ",")[0], true
}

// TestEveryFieldIsTagged: a field without a json tag would go over the wire
// under its Go name, which is not the camelCase the fixtures speak (C-6.1).
func TestEveryFieldIsTagged(t *testing.T) {
	for name, st := range structs(t) {
		for _, f := range st.Fields.List {
			if len(f.Names) == 0 { // embedded: flattened, carries no tag
				if f.Tag != nil {
					t.Errorf("%s: embedded field carries a json tag", name)
				}
				continue
			}
			tag, ok := tagOf(f)
			switch {
			case !ok:
				t.Errorf("%s.%s: no json tag", name, f.Names[0].Name)
			case tag == "" || tag == "-":
				t.Errorf("%s.%s: json tag %q hides the field", name, f.Names[0].Name, tag)
			case unicode.IsUpper(rune(tag[0])):
				t.Errorf("%s.%s: json tag %q does not start lowercase", name, f.Names[0].Name, tag)
			}
		}
	}
}

// TestNoShadowedTags: encoding/json resolves a tag collision by depth, so a
// name used both on a page and in the Shell it embeds silently drops the
// deeper field. Nothing reports it — neither the compiler nor a round-trip
// over a fixture that happens not to set it.
func TestNoShadowedTags(t *testing.T) {
	all := structs(t)

	var collect func(name, path string, seen map[string]string, depth int)
	collect = func(name, path string, seen map[string]string, depth int) {
		st, ok := all[name]
		if !ok || depth > 8 {
			return
		}
		for _, f := range st.Fields.List {
			if len(f.Names) == 0 {
				id, ok := f.Type.(*ast.Ident)
				if !ok {
					continue
				}
				collect(id.Name, path+id.Name+".", seen, depth+1)
				continue
			}
			tag, ok := tagOf(f)
			if !ok || tag == "" {
				continue // reported by TestEveryFieldIsTagged
			}
			field := path + f.Names[0].Name
			if prev, dup := seen[tag]; dup {
				t.Errorf("%s: json tag %q on both %s and %s", name, tag, prev, field)
				continue
			}
			seen[tag] = field
		}
	}
	for name := range all {
		collect(name, "", map[string]string{}, 0)
	}
}

// TestPointerFieldsKeepTheirKey: fixtures/README.md fixes the shape of an
// absent value — `null` where C-6 has a pointer, a missing key where it has
// none. `omitempty` on a pointer drops the key instead, so a fixture that
// spells the absence out no longer round-trips.
func TestPointerFieldsKeepTheirKey(t *testing.T) {
	for name, st := range structs(t) {
		for _, f := range st.Fields.List {
			if len(f.Names) == 0 {
				continue
			}
			if _, ptr := f.Type.(*ast.StarExpr); !ptr {
				continue
			}
			if f.Tag == nil {
				continue // reported by TestEveryFieldIsTagged
			}
			lit, err := strconv.Unquote(f.Tag.Value)
			if err != nil {
				continue
			}
			v, _ := reflect.StructTag(lit).Lookup("json")
			for _, opt := range strings.Split(v, ",")[1:] {
				if opt == "omitempty" {
					t.Errorf("%s.%s: pointer field is omitempty; absence is null, not a missing key",
						name, f.Names[0].Name)
				}
			}
		}
	}
}
