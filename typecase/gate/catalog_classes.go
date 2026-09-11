package gate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var tcAnyClass = regexp.MustCompile(`\.tc-([a-z][a-z0-9_-]+)`)
var tcHTMLClass = regexp.MustCompile(`\btc-([a-z][a-z0-9_-]+)\b`)

// CatalogClassHooks lists tc-* element classes that appear in catalog markup
// without a dedicated stylesheet rule. Each value explains why no rule is needed.
var CatalogClassHooks = map[string]string{}

func checkCatalogElementClasses(p ModulePaths, catalog []CatalogEntry) error {
	cssClasses, err := cssAllClasses(p.CSSDir)
	if err != nil {
		return err
	}
	used := catalogElementClasses(catalog)
	var missing []string
	for cls := range used {
		if cssClasses[cls] {
			continue
		}
		if _, ok := CatalogClassHooks[cls]; ok {
			continue
		}
		missing = append(missing, cls)
	}
	if len(missing) > 0 {
		return fmt.Errorf("completeness: catalog classes without css or hook: %s", strings.Join(missing, ", "))
	}
	return nil
}

func catalogElementClasses(catalog []CatalogEntry) map[string]bool {
	found := make(map[string]bool)
	for _, e := range catalog {
		for _, m := range tcHTMLClass.FindAllStringSubmatch(e.HTML, -1) {
			cls := "tc-" + m[1]
			if strings.Contains(m[1], "__") {
				found[cls] = true
			}
		}
	}
	return found
}

func cssAllClasses(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	found := make(map[string]bool)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".css") || e.Name() == "00-tokens.css" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		for _, m := range tcAnyClass.FindAllStringSubmatch(string(data), -1) {
			found["tc-"+m[1]] = true
		}
	}
	return found, nil
}
