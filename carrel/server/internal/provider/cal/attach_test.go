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

func attachEncoded(t *testing.T, obj *model.Object) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.String()
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
