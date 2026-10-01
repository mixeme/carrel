//go:build integration

package integration

import (
	"strings"

	"carrel.dev/carrel/server/view"
)

// runsText is a sentence of the server in pieces (C-6.2 Runs) read as plain
// words: the live tests look for what it says, not how it is marked.
func runsText(r view.Runs) string {
	var b strings.Builder
	for _, p := range r {
		b.WriteString(string(p.Text))
	}
	return b.String()
}
