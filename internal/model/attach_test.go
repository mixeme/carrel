// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
)

const journalWithAttachments = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//jtx Board//EN
BEGIN:VJOURNAL
UID:note-1
DTSTAMP:20260811T090000Z
DTSTART;VALUE=DATE:20260811
SUMMARY:Kitchen rebuild
X-JTX-COLOR:#ff0000
ATTACH;FMTTYPE=image/png;FILENAME=plan.png;SIZE=20481:https://dav.example/fil
 es/2026-08-11-kitchen.png
ATTACH;FMTTYPE=text/plain;ENCODING=BASE64;VALUE=BINARY:aGVsbG8gdGhlcmU=
END:VJOURNAL
END:VCALENDAR
`

func TestAttachmentsAreReadWithTheirParameters(t *testing.T) {
	obj, err := ParseICal("/c/note-1.ics", `"1"`, []byte(journalWithAttachments))
	if err != nil {
		t.Fatalf("ParseICal: %v", err)
	}
	list := obj.Attachments()
	if len(list) != 2 {
		t.Fatalf("attachments = %d, want 2", len(list))
	}

	link := list[0]
	if link.Inline {
		t.Fatal("a URI attachment was reported as inline")
	}
	if link.URI != "https://dav.example/files/2026-08-11-kitchen.png" {
		t.Fatalf("uri = %q (unfolding of the continuation line failed?)", link.URI)
	}
	if link.FmtType != "image/png" {
		t.Fatalf("fmttype = %q", link.FmtType)
	}
	if link.Filename != "plan.png" {
		t.Fatalf("filename = %q, want the FILENAME parameter to win over the URI", link.Filename)
	}
	if !link.HasSize || link.Size != 20481 {
		t.Fatalf("size = %d (has=%v)", link.Size, link.HasSize)
	}
	if got := link.SizeLabel(); got != "20 kB" {
		t.Fatalf("size label = %q", got)
	}

	// §23.10: an attachment another client embedded is shown as it is. It has no
	// URI to open and is marked so the interface can say why.
	inline := list[1]
	if !inline.Inline {
		t.Fatal("a base64 attachment was not reported as inline")
	}
	if inline.URI != "" {
		t.Fatalf("inline attachment reported a URI %q", inline.URI)
	}
}

func TestAttachmentFilenameFallsBackToTheURI(t *testing.T) {
	att := attachmentFrom(Value{Text: "https://dav.example/files/sub/%D0%9E%D1%82%D1%87%D1%91%D1%82.pdf"})
	if att.Filename != "Отчёт.pdf" {
		t.Fatalf("filename = %q, want the last segment percent-decoded", att.Filename)
	}
	if got := att.DisplayName(); got != "Отчёт.pdf" {
		t.Fatalf("display name = %q", got)
	}
}

// ATTACH is a property this build now renders, so it must not also show up among
// the properties the card lists as untouched foreign ones — and X-JTX-COLOR must
// still be there, because that one really is foreign (§8, §23.9).
func TestAttachIsModelledAndOtherPropertiesAreNot(t *testing.T) {
	obj, err := ParseICal("/c/note-1.ics", "", []byte(journalWithAttachments))
	if err != nil {
		t.Fatal(err)
	}
	note, err := obj.Note(time.UTC)
	if err != nil {
		t.Fatalf("Note: %v", err)
	}
	if len(note.Attachments) != 2 {
		t.Fatalf("note attachments = %d", len(note.Attachments))
	}
	for _, prop := range note.Other {
		if prop.Name == ical.PropAttach {
			t.Fatal("ATTACH appeared in Other as well as in Attachments")
		}
	}
	if !hasProperty(note.Other, "X-JTX-COLOR") {
		t.Fatal("X-JTX-COLOR was lost from Other")
	}
}

// Detaching one attachment must put the others back exactly as they came,
// including the inline one §23.10 forbids rewriting into a link.
func TestDetachRewritesTheSetAndPreservesTheRest(t *testing.T) {
	obj, err := ParseICal("/c/note-1.ics", "", []byte(journalWithAttachments))
	if err != nil {
		t.Fatal(err)
	}
	list := obj.Attachments()
	kept := []Attachment{list[1]}
	if err := obj.Apply(AttachPatch(&Patch{}, kept)); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	body, err := obj.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	unfolded := unfold(string(body))
	if strings.Contains(unfolded, "2026-08-11-kitchen.png") {
		t.Fatal("the detached attachment is still on the object")
	}
	if !strings.Contains(unfolded, "aGVsbG8gdGhlcmU=") {
		t.Fatal("the inline attachment was dropped")
	}
	if !strings.Contains(unfolded, "ENCODING=BASE64") || !strings.Contains(unfolded, "VALUE=BINARY") {
		t.Fatalf("the inline attachment lost its parameters:\n%s", unfolded)
	}
	if !strings.Contains(unfolded, "X-JTX-COLOR:#ff0000") {
		t.Fatal("editing the attachments dropped an unrelated property")
	}
}

// Detaching the last one removes the property rather than writing an empty
// value, which is a different thing to every other client (§11).
func TestDetachingTheLastAttachmentRemovesTheProperty(t *testing.T) {
	obj, err := ParseICal("/c/note-1.ics", "", []byte(journalWithAttachments))
	if err != nil {
		t.Fatal(err)
	}
	if err := obj.Apply(AttachPatch(&Patch{}, nil)); err != nil {
		t.Fatal(err)
	}
	if obj.Has(ical.PropAttach) {
		t.Fatal("ATTACH is still present after removing every attachment")
	}
	body, _ := obj.Marshal()
	if strings.Contains(string(body), "ATTACH") {
		t.Fatalf("serialised object still mentions ATTACH:\n%s", body)
	}
}

func TestAttachmentValueRoundTrips(t *testing.T) {
	obj, err := NewJournal("note-2")
	if err != nil {
		t.Fatal(err)
	}
	v := AttachmentValue("https://dav.example/files/a b.png", "image/png", "a b.png", 1024)
	if err := obj.Apply((&Patch{}).Set(ical.PropAttach, v)); err != nil {
		t.Fatal(err)
	}
	body, err := obj.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	reparsed, err := ParseICal("/c/note-2.ics", "", body)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	list := reparsed.Attachments()
	if len(list) != 1 {
		t.Fatalf("attachments after a round trip = %d", len(list))
	}
	got := list[0]
	if got.URI != "https://dav.example/files/a b.png" || got.Filename != "a b.png" ||
		got.FmtType != "image/png" || !got.HasSize || got.Size != 1024 {
		t.Fatalf("round trip lost something: %+v", got)
	}
	// A round trip through the server-comparison path reports no loss, which is
	// what §8 checks after every write.
	loss, err := Compare(obj, reparsed)
	if err != nil {
		t.Fatal(err)
	}
	if !loss.Empty() {
		t.Fatalf("round trip reported a loss: %s", loss.String())
	}
}

// A Markdown export carries the links and an import puts them back, so the
// substance of an attachment survives the round trip §23.9 promises.
func TestMarkdownCarriesAttachmentLinks(t *testing.T) {
	obj, err := ParseICal("/c/note-1.ics", "", []byte(journalWithAttachments))
	if err != nil {
		t.Fatal(err)
	}
	note, err := obj.Note(time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	out := string(RenderMarkdown(note, MarkdownSource{Account: "Home", Collection: "Journal"}))
	if !strings.Contains(out, "attachments:") ||
		!strings.Contains(out, "https://dav.example/files/2026-08-11-kitchen.png") {
		t.Fatalf("export carries no attachment link:\n%s", out)
	}
	// The inline one has no link and is not carried, which is stated rather than
	// silently done.
	if strings.Contains(out, "aGVsbG8gdGhlcmU=") {
		t.Fatal("base64 attachment data was written into the Markdown front matter")
	}

	parsed, err := ParseMarkdown("note.md", []byte(out), time.Time{})
	if err != nil {
		t.Fatalf("ParseMarkdown: %v", err)
	}
	if len(parsed.Attachments) != 1 {
		t.Fatalf("imported attachments = %v", parsed.Attachments)
	}
	fresh, err := NewJournal("note-3")
	if err != nil {
		t.Fatal(err)
	}
	if err := fresh.Apply(parsed.Patch(time.UTC)); err != nil {
		t.Fatal(err)
	}
	list := fresh.Attachments()
	if len(list) != 1 || list[0].URI != "https://dav.example/files/2026-08-11-kitchen.png" {
		t.Fatalf("attachment after import = %+v", list)
	}
	if list[0].FmtType != "image/png" || list[0].Filename != "2026-08-11-kitchen.png" {
		t.Fatalf("import did not fill in the name and type: %+v", list[0])
	}
}

func TestEventAttachmentsAreModelled(t *testing.T) {
	const body = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Carrel//EN
BEGIN:VEVENT
UID:ev-1
DTSTAMP:20260811T090000Z
DTSTART:20260811T100000Z
DTEND:20260811T110000Z
SUMMARY:Site visit
ATTACH;FMTTYPE=application/pdf:https://dav.example/files/site.pdf
END:VEVENT
END:VCALENDAR
`
	obj, err := ParseICal("/c/ev-1.ics", "", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	ev, err := obj.Event(time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(ev.Attachments) != 1 || ev.Attachments[0].Filename != "site.pdf" {
		t.Fatalf("event attachments = %+v", ev.Attachments)
	}
	if hasProperty(ev.Other, ical.PropAttach) {
		t.Fatal("ATTACH is both modelled and listed as a foreign property")
	}
}

func TestByteSize(t *testing.T) {
	cases := map[int64]string{
		0: "0 B", 512: "512 B", 1024: "1.0 kB", 20481: "20 kB",
		1 << 20: "1.0 MB", 5 << 20: "5.0 MB", 1 << 30: "1.0 GB",
	}
	for in, want := range cases {
		if got := ByteSize(in); got != want {
			t.Fatalf("ByteSize(%d) = %q, want %q", in, got, want)
		}
	}
}

func hasProperty(props []Property, name string) bool {
	for _, prop := range props {
		if prop.Name == name {
			return true
		}
	}
	return false
}
