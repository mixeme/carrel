package cal

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/model"
)

const pngData = "PNGDATA"

func attachDoc(t *testing.T) *model.Object {
	t.Helper()
	doc := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other client//EN\r\nBEGIN:VEVENT\r\nUID:e1\r\n" +
		"DTSTAMP:20260101T000000Z\r\nDTSTART:20260810T100000Z\r\nSUMMARY:Sync\r\n" +
		"ATTACH;FMTTYPE=text/markdown;FILENAME=notes.md;SIZE=3072:https://files.example.org/carrel/notes.md\r\n" +
		"ATTACH;ENCODING=BASE64;VALUE=BINARY;FMTTYPE=image/png;X-APPLE-FILENAME=shot.png:" +
		base64.StdEncoding.EncodeToString([]byte(pngData)) + "\r\n" +
		"END:VEVENT\r\nEND:VCALENDAR\r\n"
	obj, err := Parse("/c/e1.ics", "etag-1", strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	return obj
}

// attachEncoded is the record as it goes out, unfolded: what is written is
// folded at 75 octets (SV-31), and an assertion here is about what the record
// holds, not about how the line was broken. TestEncodeFoldsLongLines holds the
// folding itself.
func attachEncoded(t *testing.T, obj *model.Object) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return unfolded(buf.String())
}

// unfolded is what a parser does with a fold: a CRLF and one space or tab are
// not there (RFC 5545 §3.1).
func unfolded(doc string) string {
	doc = strings.ReplaceAll(doc, "\r\n ", "")
	return strings.ReplaceAll(doc, "\r\n\t", "")
}

func attachApply(t *testing.T, obj *model.Object, ops []model.PatchOp) *model.Object {
	t.Helper()
	patch, err := model.NewPatch(ops...)
	if err != nil {
		t.Fatal(err)
	}
	if err := patch.Apply(obj); err != nil {
		t.Fatal(err)
	}
	return obj
}

// S-20: an attachment is either the file itself or a link to it, and Carrel
// reads both as another client wrote them — the name from whichever parameter
// carries it, the size of inline data from the data.
func TestAttachmentsAreReadAsStored(t *testing.T) {
	list := Attachments(attachDoc(t))
	if len(list) != 2 {
		t.Fatalf("attachments: %+v", list)
	}
	link, inline := list[0], list[1]
	if link.Inline || link.URI != "https://files.example.org/carrel/notes.md" || link.FileName != "notes.md" ||
		link.Type != "text/markdown" || link.Size != 3072 {
		t.Errorf("link = %+v", link)
	}
	if !inline.Inline || inline.URI != "" || inline.FileName != "shot.png" || inline.Type != "image/png" ||
		inline.Size != int64(len(pngData)) || inline.Index != 1 {
		t.Errorf("inline = %+v", inline)
	}

	a, data, err := AttachmentData(attachDoc(t), 1)
	if err != nil || string(data) != pngData || a.FileName != "shot.png" {
		t.Errorf("data = %q %+v %v", data, a, err)
	}
	if _, _, err := AttachmentData(attachDoc(t), 0); err != ErrNoAttachment {
		t.Errorf("a link has no data here: %v", err)
	}
	if _, _, err := AttachmentData(attachDoc(t), 7); err != ErrNoAttachment {
		t.Errorf("no such attachment: %v", err)
	}
	if list := Attachments(attachApply(t, attachDoc(t), []model.PatchOp{model.RemoveProp{Prop: "ATTACH"}})); list != nil {
		t.Errorf("a record without attachments: %+v", list)
	}
}

// Adding one leaves every attachment already there as it was written.
func TestAttachInlineAndLinkAppend(t *testing.T) {
	obj := attachDoc(t)
	doc := attachEncoded(t, attachApply(t, obj, AttachInline(obj, "report.pdf", "application/pdf", []byte("PDF"))))
	want := "ATTACH;ENCODING=BASE64;FILENAME=report.pdf;FMTTYPE=application/pdf;VALUE=BINARY:" +
		base64.StdEncoding.EncodeToString([]byte("PDF"))
	if !strings.Contains(doc, want) {
		t.Errorf("no inline attachment in\n%s", doc)
	}
	for _, kept := range []string{"FILENAME=notes.md", "X-APPLE-FILENAME=shot.png", "SIZE=3072"} {
		if !strings.Contains(doc, kept) {
			t.Errorf("%s was lost:\n%s", kept, doc)
		}
	}

	obj = attachDoc(t)
	doc = attachEncoded(t, attachApply(t, obj, AttachLink(obj, "https://files.example.org/carrel/big.mp4", "big.mp4", "video/mp4", 4096)))
	if !strings.Contains(doc, "ATTACH;FILENAME=big.mp4;FMTTYPE=video/mp4;SIZE=4096:https://files.example.org/carrel/big.mp4") {
		t.Errorf("no link in\n%s", doc)
	}
}

// A name with a double quote has no escape in a parameter (RFC 5545 §3.2), so
// it must not reach the encoder as one.
func TestAttachNameWithAQuoteIsStillWritten(t *testing.T) {
	obj := attachDoc(t)
	doc := attachEncoded(t, attachApply(t, obj, AttachInline(obj, `say "hi";now.txt`, "text/plain", []byte("x"))))
	if !strings.Contains(doc, `FILENAME="say 'hi';now.txt"`) {
		t.Errorf("name not written:\n%s", doc)
	}
}

// Removing one keeps the others byte for byte: parameters another client
// wrote are not a thing Carrel rewrites (S-7).
func TestDetachKeepsTheOthers(t *testing.T) {
	obj := attachDoc(t)
	ops, err := Detach(obj, 0)
	if err != nil {
		t.Fatal(err)
	}
	doc := attachEncoded(t, attachApply(t, obj, ops))
	if strings.Contains(doc, "notes.md") {
		t.Errorf("the link stayed:\n%s", doc)
	}
	if !strings.Contains(doc, "ATTACH;ENCODING=BASE64;FMTTYPE=image/png;VALUE=BINARY;X-APPLE-FILENAME=shot.png:"+
		base64.StdEncoding.EncodeToString([]byte(pngData))) {
		t.Errorf("the inline attachment changed:\n%s", doc)
	}
	if _, err := Detach(obj, 9); err != ErrNoAttachment {
		t.Errorf("no such attachment: %v", err)
	}

	obj = attachDoc(t)
	ops, _ = Detach(obj, 1)
	if doc := attachEncoded(t, attachApply(t, obj, ops)); strings.Contains(doc, "shot.png") || !strings.Contains(doc, "notes.md") {
		t.Errorf("the inline one should be gone and the link stay:\n%s", doc)
	}
}

// docs/servers.md, SV-31: go-ical writes a property of any length on one line,
// so Carrel folds what goes out. An inline attachment is the case that made it
// matter: hundreds of kilobytes of base64 on a single line.
func TestEncodeFoldsLongLines(t *testing.T) {
	obj := attachDoc(t)
	big := make([]byte, 60<<10)
	for i := range big {
		big[i] = byte('a' + i%26)
	}
	obj = attachApply(t, obj, AttachInline(obj, "big.bin", "application/octet-stream", big))
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		t.Fatal(err)
	}
	doc := buf.String()
	if len(doc) < 60<<10 {
		t.Fatalf("the attachment did not get in: %d octets", len(doc))
	}
	for i, line := range strings.Split(strings.TrimSuffix(doc, "\r\n"), "\r\n") {
		if len(line) > 75 {
			t.Fatalf("line %d is %d octets", i+1, len(line))
		}
	}
	// And it is the same record: unfolded, the attachment is whole.
	back, err := Parse("/c/e.ics", "", strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	_, data, err := AttachmentData(back, len(Attachments(back))-1)
	if err != nil || !bytes.Equal(data, big) {
		t.Errorf("the attachment came back as %d octets, err %v", len(data), err)
	}
}

// Stalwart keeps an inline file that reads as text as text: VALUE=BINARY
// without ENCODING=BASE64, the value escaped as TEXT (docs/servers.md,
// SV-67). Read as base64 it was three quarters of its size and its data was
// noise (acceptance on our test servers, 18.11.2026). A value of base64
// without the parameter is still base64.
func TestAnInlineFileKeptAsTextIsItsText(t *testing.T) {
	doc := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Stalwart//EN\r\nBEGIN:VEVENT\r\nUID:e2\r\n" +
		"DTSTAMP:20260101T000000Z\r\nDTSTART:20260810T100000Z\r\nSUMMARY:Sync\r\n" +
		`ATTACH;FILENAME=release.md;FMTTYPE=text/markdown;VALUE=BINARY:# Release\n\n- check the build\, sign off\; done \\ ok` + "\r\n" +
		"ATTACH;VALUE=BINARY;FMTTYPE=image/png:" + base64.StdEncoding.EncodeToString([]byte(pngData)) + "\r\n" +
		"END:VEVENT\r\nEND:VCALENDAR\r\n"
	obj, err := Parse("/c/e2.ics", "etag-1", strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	want := "# Release\n\n- check the build, sign off; done \\ ok"
	list := Attachments(obj)
	if len(list) != 2 || !list[0].Inline || list[0].Size != int64(len(want)) || list[1].Size != int64(len(pngData)) {
		t.Fatalf("attachments = %+v, want sizes %d and %d", list, len(want), len(pngData))
	}
	if _, data, err := AttachmentData(obj, 0); err != nil || string(data) != want {
		t.Errorf("text kept as text = %q, %v; want %q", data, err, want)
	}
	if _, data, err := AttachmentData(obj, 1); err != nil || string(data) != pngData {
		t.Errorf("base64 without ENCODING = %q, %v", data, err)
	}
}
