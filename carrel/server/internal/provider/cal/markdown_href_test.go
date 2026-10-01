package cal

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// A link of a note goes only where a link may go from someone else's text:
// http, https, mailto, tel (C-6.9, docs/security.md «Ссылки из текста
// записи»). The note is written by whoever writes to the collection, and
// public notes (/p/) show it to anyone; a javascript: link on the page is the
// page's own script. Anything else is the link's words without a link.
func TestRenderMarkdownLinkSchemes(t *testing.T) {
	refused := []string{
		"[a](javascript:alert(1))",
		"[a](JaVaScRiPt:alert(1))",
		"[a](javascript&#58;alert(1))",
		"[a](data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==)",
		"[a](vbscript:msgbox(1))",
		"[a](< javascript:alert(1)>)",
		"[a](<\tjavascript:alert(1)>)",
		"[a](<java\tscript:alert(1)>)",
		"[a](&#1;javascript:alert(1))",
		"[a](java%0ascript:alert(1))",
		"[a](file:///etc/passwd)",
		"[a](urn:uuid:7c1e2a40-0000-4000-8000-000000000001)",
		"[a](/admin/users)",
		"[a](//evil.example/x)",
		"[a](https:evil.example)",
		"[a][r]\n\n[r]: javascript:alert(1)",
		"<javascript:alert(1)>",
		"<JAVASCRIPT:alert(1)>",
		"<data:text/html,a>",
	}
	for _, src := range refused {
		blocks := RenderMarkdown(src)
		if hrefs := blockHrefs(blocks); len(hrefs) != 0 {
			t.Errorf("%q: links %q, want none", src, hrefs)
		}
		if kinds := blockKinds(blocks); containsKind(kinds, view.BlockLink) {
			t.Errorf("%q: a link block without a link: %+v", src, blocks)
		}
		words := blockWords(blocks)
		want := "a"
		if strings.HasPrefix(src, "<") {
			want = strings.Trim(src, "<>")
		}
		if !strings.Contains(words, want) {
			t.Errorf("%q: words %q, want the link's own %q shown", src, words, want)
		}
	}

	kept := map[string]string{
		"[a](https://example.com/x?y=1)": "https://example.com/x?y=1",
		"[a](http://example.com)":        "http://example.com",
		"[a](HTTPS://EXAMPLE.COM)":       "HTTPS://EXAMPLE.COM",
		"[a](mailto:anna@example.com)":   "mailto:anna@example.com",
		"[a](tel:+79124451088)":          "tel:+79124451088",
		"<https://example.com>":          "https://example.com",
		"<anna@example.com>":             "mailto:anna@example.com",
	}
	for src, want := range kept {
		hrefs := blockHrefs(RenderMarkdown(src))
		if len(hrefs) != 1 || hrefs[0] != want {
			t.Errorf("%q: links %q, want [%q]", src, hrefs, want)
		}
	}
}

func blockHrefs(blocks []view.Block) []string {
	var out []string
	for _, b := range blocks {
		if b.Href != "" {
			out = append(out, b.Href)
		}
		out = append(out, blockHrefs(b.Children)...)
	}
	return out
}

func blockKinds(blocks []view.Block) []view.BlockKind {
	var out []view.BlockKind
	for _, b := range blocks {
		out = append(out, b.Kind)
		out = append(out, blockKinds(b.Children)...)
	}
	return out
}

func blockWords(blocks []view.Block) string {
	var b strings.Builder
	for _, x := range blocks {
		b.WriteString(string(x.Text))
		b.WriteString(blockWords(x.Children))
	}
	return b.String()
}
