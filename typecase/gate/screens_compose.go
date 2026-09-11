package gate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"carrel.dev/typecase/screens"
)

var notesComposeBypass = []string{
	`<main class="tc-main"`,
	`<div class="tc-bar"`,
	`<aside class="tc-side"`,
}

// CheckScreensCompose rejects hand-built tc-main, tc-bar, and tc-side markup in notes.go.
func CheckScreensCompose(p ModulePaths) error {
	path := filepath.Join(p.Root, "screens", "notes.go")
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("screens-compose: read notes.go: %w", err)
	}
	text := string(src)
	var found []string
	for _, pat := range notesComposeBypass {
		if strings.Contains(text, pat) {
			found = append(found, pat)
		}
	}
	if len(found) > 0 {
		return fmt.Errorf("screens-compose: notes.go bypasses widgets: %s", strings.Join(found, ", "))
	}
	html := screens.Notes()
	if strings.Contains(html, "tc-row--excerpt") {
		if !strings.Contains(html, "tc-row__date") || !strings.Contains(html, "tc-row__source") {
			return fmt.Errorf("screens-compose: notes excerpt rows must include date and source cells")
		}
	}
	return nil
}
