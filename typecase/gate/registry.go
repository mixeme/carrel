package gate

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// WidgetSpec is one widget from L-3.2 comments.
type WidgetSpec struct {
	GoType   string
	Name     string
	States   []string
	Sections map[string]string
}

var (
	widgetHeader     = regexp.MustCompile(`^// (\w+) — (tc-[a-z][a-z0-9-]*)`)
	sectionLine      = regexp.MustCompile(`^// (Роль|Вход|Состояния|Инварианты|Поведение|Геометрия|Содержимое|Чего нет)\.`)
	requiredSections = []string{
		"Роль", "Вход", "Состояния", "Инварианты", "Поведение", "Геометрия", "Содержимое", "Чего нет",
	}
)

// LoadWidgets parses widget/*.go L-3.2 headers.
func LoadWidgets(widgetDir string) (map[string]WidgetSpec, error) {
	entries, err := os.ReadDir(widgetDir)
	if err != nil {
		return nil, err
	}
	out := make(map[string]WidgetSpec)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		if e.Name() == "doc.go" || e.Name() == "escape.go" || e.Name() == "types.go" {
			continue
		}
		path := filepath.Join(widgetDir, e.Name())
		specs, err := parseWidgetFile(path)
		if err != nil {
			return nil, err
		}
		for name, spec := range specs {
			out[name] = spec
		}
	}
	return out, nil
}

func parseWidgetFile(path string) (map[string]WidgetSpec, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := make(map[string]WidgetSpec)
	var cur *WidgetSpec
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if m := widgetHeader.FindStringSubmatch(line); m != nil {
			if cur != nil && cur.Name != "" {
				out[cur.Name] = *cur
			}
			cur = &WidgetSpec{
				GoType:   m[1],
				Name:     m[2],
				Sections: make(map[string]string),
			}
			continue
		}
		if cur == nil {
			continue
		}
		if m := sectionLine.FindStringSubmatch(line); m != nil {
			text := strings.TrimSpace(strings.TrimPrefix(line, "// "+m[1]+"."))
			cur.Sections[m[1]] = text
			if m[1] == "Состояния" {
				cur.States = parseStates(text)
			}
		}
	}
	if cur != nil && cur.Name != "" {
		out[cur.Name] = *cur
	}
	return out, sc.Err()
}

func parseStates(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" || strings.HasPrefix(text, "нет") {
		return []string{"default"}
	}
	if strings.HasPrefix(text, "по роду") || strings.HasPrefix(text, "участников:") {
		return []string{"default"}
	}
	if strings.Contains(text, ";") {
		text = strings.Split(text, ";")[0]
	}
	parts := strings.Split(text, "|")
	var states []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if i := strings.Index(p, " "); i > 0 {
			p = p[:i]
		}
		p = strings.TrimRight(p, ".,;")
		if p == "" || strings.HasPrefix(p, "по ") {
			continue
		}
		states = append(states, slugState(p))
	}
	if len(states) == 0 {
		return []string{"default"}
	}
	return states
}

func slugState(s string) string {
	return strings.NewReplacer(" ", "", "/", "-", ".", "-").Replace(s)
}
