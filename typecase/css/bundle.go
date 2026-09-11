// Package css concatenates stylesheet fragments in declared order at startup (L-11.1).
package css

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"

	"carrel.dev/typecase/token"
)

//go:embed *.css
var files embed.FS

// Order is the concatenation order; changing it changes cascade precedence.
var Order = []string{
	"00-tokens.css",
	"01-base.css",
	"02-primitives.css",
	"03-forms.css",
	"04-overlays.css",
	"05-panels.css",
	"06-lists.css",
	"07-tables.css",
	"08-shell.css",
	"09-document.css",
	"10-utility.css",
}

// Bundle reads fragments in Order, joins them, and validates required tokens.
func Bundle() (string, error) {
	var b strings.Builder
	for _, name := range Order {
		data, err := files.ReadFile(name)
		if err != nil {
			return "", fmt.Errorf("css: read %s: %w", name, err)
		}
		b.Write(data)
		if !strings.HasSuffix(string(data), "\n") {
			b.WriteByte('\n')
		}
	}
	out := b.String()
	if err := token.Validate(out); err != nil {
		return "", err
	}
	return out, nil
}

// MustBundle is Bundle that panics on error; for init-time wiring only.
func MustBundle() string {
	s, err := Bundle()
	if err != nil {
		panic(err)
	}
	return s
}

// Files returns the embedded CSS filesystem for static serving.
func Files() fs.FS {
	return files
}
