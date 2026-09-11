package gate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"carrel.dev/typecase/screens"
)

var screensComposeBypass = []string{
	`<main class="tc-main"`,
	`<div class="tc-shell"`,
	`<div class="tc-bar"`,
	`<aside class="tc-side"`,
}

// CheckScreensCompose rejects hand-built shell markup in demo screen assemblers.
func CheckScreensCompose(p ModulePaths) error {
	screensDir := filepath.Join(p.Root, "screens")
	entries, err := os.ReadDir(screensDir)
	if err != nil {
		return fmt.Errorf("screens-compose: read screens: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		switch e.Name() {
		case "fixtures.go", "serve.go", "shell_common.go", "style.go", "widgets.go":
			continue
		}
		path := filepath.Join(screensDir, e.Name())
		src, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("screens-compose: read %s: %w", e.Name(), err)
		}
		text := string(src)
		var found []string
		for _, pat := range screensComposeBypass {
			if strings.Contains(text, pat) {
				found = append(found, pat)
			}
		}
		if len(found) > 0 {
			return fmt.Errorf("screens-compose: %s bypasses widgets: %s", e.Name(), strings.Join(found, ", "))
		}
	}
	html := screens.Notes()
	if strings.Contains(html, "tc-row--excerpt") {
		if !strings.Contains(html, "tc-row__date") || !strings.Contains(html, "tc-row__source") {
			return fmt.Errorf("screens-compose: notes excerpt rows must include date and source cells")
		}
	}
	list := screens.Notes()
	if strings.Contains(list, "tc-doc") {
		return fmt.Errorf("screens-compose: notes list must not include tc-doc preview")
	}
	read := screens.NotesRead()
	if !strings.Contains(read, "tc-tree") {
		return fmt.Errorf("screens-compose: notes-read must include tc-tree neighbours")
	}
	quick := screens.NotesQuick()
	if !strings.Contains(quick, "tc-sheet is-open") {
		return fmt.Errorf("screens-compose: notes-quick must have open sheet")
	}
	edit := screens.NotesEdit()
	if strings.Contains(edit, "tc-shell__rail") {
		return fmt.Errorf("screens-compose: notes-edit must not include rail column")
	}
	if strings.Contains(edit, "tc-side") {
		return fmt.Errorf("screens-compose: notes-edit must not include side panel")
	}
	return nil
}
