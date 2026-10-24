package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Text and Markdown are the body of their public page (Пс14; s-publish-10):
// Markdown parsed into blocks, plain text as it is, Download then a second
// door; a file under a text name that holds a NUL is offered to download,
// as is a text over the ceiling. The foot says until when, and how the file
// is read.
func TestPublishedTextIsItsBody(t *testing.T) {
	f := pubDocs(t)
	f.srv.SeedFile(filesDocs+"plan.md", "## Before\n\n- run the tests\n- check the changelog\n")
	f.srv.SeedFile(filesDocs+"plain.txt", "one line\nanother")
	f.srv.SeedFile(filesDocs+"fake.txt", "text\x00and a NUL")
	f.srv.SeedFile(filesDocs+"big.md", strings.Repeat("a", 256<<10+1))

	md := publicPage(t, f, publishFile(t, f, "plan.md", false))
	if md.File == nil || md.File.Preview != view.PreviewMarkdown || len(md.File.Blocks) != 2 || md.File.Blocks[0].Kind != view.BlockHeading {
		t.Fatalf("a Markdown file: %+v", md.File)
	}
	if a := md.Actions[0]; a.Primary || a.Label != "Download" {
		t.Errorf("under a text read on the page Download is %+v", a)
	}
	// The foot is one phrase on every page (note 122 p. 3).
	if foot := runsText(md.Foot); !strings.HasPrefix(foot, "This link works until ") || !strings.HasSuffix(foot, ".") || strings.Contains(foot, "read from the server") {
		t.Errorf("foot = %q", foot)
	}
	plain := publicPage(t, f, publishFile(t, f, "plain.txt", false))
	if plain.File.Preview != view.PreviewText || plain.File.Body != "one line\nanother" {
		t.Errorf("a plain text: %+v", plain.File)
	}
	for _, name := range []string{"fake.txt", "big.md"} {
		page := publicPage(t, f, publishFile(t, f, name, false))
		if page.File.Preview != view.PreviewNone || page.File.Body != "" || len(page.File.Blocks) != 0 || !page.Actions[0].Primary || page.File.Note != "Too large to show on the page." {
			t.Errorf("%s is offered to download: %+v", name, page.File)
		}
		if foot := runsText(page.Foot); strings.Contains(foot, "read from the server") {
			t.Errorf("%s: foot = %q", name, foot)
		}
	}
}
