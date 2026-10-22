package web_test

import (
	"net/url"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The rows of what a letter carries say which one (Пб21; s-mail-1): a
// contact where they work, a folder how much is in it, a file its name and
// size with no format beside the name that already has it. The link a
// folder goes as is named by the folder alone.
func TestLetterRowsSayWhichOne(t *testing.T) {
	f, _ := letterFixture(t)
	f.srv.Seed(searchBook+"grace.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:grace\nFN:Grace Hopper\nORG:Navy\nEND:VCARD\n")
	attachOne(t, f, url.Values{"what": {"contact"}, "a": {f.id}, "c": {searchBook}, "uid": {"grace"}})
	attachOne(t, f, url.Values{"what": {"folder"}, "a": {f.id}, "c": {filesDocs}, "dir": {"share"}})
	attachOne(t, f, url.Values{"what": {"file"}, "a": {f.id}, "c": {filesDocs}, "path": {"small.txt"}})
	sheet := decode[view.MailSheet](t, f.h.do(get("/app/mail/compose", f.sess)))
	rows := map[view.Text]view.MailAttachRow{}
	for _, r := range sheet.Attach {
		rows[r.Title] = r
	}
	if r, ok := rows["Grace Hopper · Navy"]; !ok || r.Format != "vcf" {
		t.Errorf("the contact's row: %+v", sheet.Attach)
	}
	if r, ok := rows["share · 1 file"]; !ok || r.Format != "folder" || !r.AsLink {
		t.Errorf("the folder's row: %+v", sheet.Attach)
	}
	if r, ok := rows["small.txt"]; !ok || r.Format != "" || r.Size == "" {
		t.Errorf("the file's row: %+v", sheet.Attach)
	}
}
