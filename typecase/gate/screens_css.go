package gate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CheckScreensCSS rejects widget selectors in screens page chrome styles.
func CheckScreensCSS(p ModulePaths) error {
	path := filepath.Join(p.Root, "screens", "screens.css")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("screens-css: read screens.css: %w", err)
	}
	if strings.Contains(string(data), ".tc-") {
		return fmt.Errorf("screens-css: screens.css must not contain .tc- selectors")
	}
	return nil
}
