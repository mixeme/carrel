package note

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

func TestSourceIsLiteralAndControlsAreLinks(t *testing.T) {
	p := &view.NotePage{Doc: view.NoteDoc{Source: true, HideSide: true, Body: "# Title\n<script>literal</script>", Controls: []view.Action{
		{ID: "note.source", Label: "Rendered", Href: "/note?view=reading", Method: "GET", Enabled: true},
		{ID: "note.width.full", Label: "Full width", Href: "/note?width=full", Method: "GET", Enabled: true},
		{ID: "note.width.read", Label: "Reading width", Href: "/note?width=read", Method: "GET", Enabled: true},
		{ID: "note.side", Label: "Show the side column", Href: "/note?side=1", Method: "GET", Enabled: true},
	}}}
	html := Screen(p).Render()
	for _, want := range []string{"# Title", "&lt;script&gt;literal&lt;/script&gt;", `href="/note?view=reading"`, `href="/note?width=full"`, `href="/note?width=read"`, `href="/note?side=1"`} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q in %s", want, html)
		}
	}
	if strings.Contains(html, "<script>literal") || strings.Contains(html, `tc-docmeta`) {
		t.Errorf("unsafe source or visible side column: %s", html)
	}
}
