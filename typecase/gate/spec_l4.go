package gate

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var tcNameInBackticks = regexp.MustCompile("`(tc-[a-z][a-z0-9-]*)`")

// LoadSpecL4Widgets parses widget names from L-4 #### headers in the spec file.
func LoadSpecL4Widgets(specPath string) (map[string]bool, error) {
	if specPath == "" {
		return nil, fmt.Errorf("spec-l4: spec path required")
	}
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("spec-l4: cannot read spec %s: %w", specPath, err)
	}
	section := extractL4Section(string(data))
	if section == "" {
		return nil, fmt.Errorf("spec-l4: L-4 section not found in %s", specPath)
	}
	declared := make(map[string]bool)
	sc := bufio.NewScanner(strings.NewReader(section))
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "#### ") {
			continue
		}
		for _, m := range tcNameInBackticks.FindAllStringSubmatch(line, -1) {
			declared[m[1]] = true
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(declared) == 0 {
		return nil, fmt.Errorf("spec-l4: no L-4 widget headers in %s", specPath)
	}
	return declared, nil
}

// CheckSpecL4 ensures implemented widgets match the L-4 registry in the spec.
func CheckSpecL4(p ModulePaths, specPath string) error {
	if specPath == "" {
		specPath = filepath.Join(p.RepoRoot, "docs", "spec", "typecase.md")
	}
	declared, err := LoadSpecL4Widgets(specPath)
	if err != nil {
		return err
	}
	widgets, err := LoadWidgets(p.WidgetDir)
	if err != nil {
		return err
	}
	var missing, extra []string
	for name := range declared {
		if _, ok := widgets[name]; !ok {
			missing = append(missing, name)
		}
	}
	for name := range widgets {
		if !declared[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	var parts []string
	if len(missing) > 0 {
		parts = append(parts, "declared not implemented: "+strings.Join(missing, ", "))
	}
	if len(extra) > 0 {
		parts = append(parts, "implemented not declared: "+strings.Join(extra, ", "))
	}
	if len(parts) > 0 {
		return fmt.Errorf("spec-l4: %s", strings.Join(parts, "; "))
	}
	return nil
}

func extractL4Section(content string) string {
	const startMark = "## L-4."
	const endMark = "## L-5."
	start := strings.Index(content, startMark)
	if start < 0 {
		return ""
	}
	rest := content[start:]
	end := strings.Index(rest, endMark)
	if end < 0 {
		return rest
	}
	return rest[:end]
}
