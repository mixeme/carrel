package web_test

import (
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/mail/mailtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
	"github.com/emersion/go-webdav/carddav"
)

// letterFixture is a person with a book (Ada, two addresses) and a file
// collection (a small file, a large one, a folder), everything ticked, mail
// on, and a relay in memory. An attachment may be 1000 bytes.
func letterFixture(t *testing.T) (davFixture, *mailtest.Server) {
	t.Helper()
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.Seed(searchBook+"ada.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEMAIL;TYPE=work:ada@example.com\nEMAIL;TYPE=home:ada@home.example\nEND:VCARD\n")
	srv.SeedFile(filesDocs+"small.txt", "hello")
	srv.SeedFile(filesDocs+"big.bin", strings.Repeat("x", 5000))
	srv.SeedFile(filesDocs+"share/inside.txt", "inside")
	f := connectFixture(t, srv, searchBook, func(h *harness) {
		h.cfg.MailEnabled = true
		h.cfg.MailMaxAttachBytes = 1000
	})
	tickAll(t, f.h, f.sess, sealed.Collection{AccountID: f.id, Path: filesDocs, Name: "Docs", Kinds: []view.Kind{view.KindFile}})
	return f, f.h.relayed()
}

func attachOne(t *testing.T, f davFixture, q url.Values) view.MailAttachSheet {
	t.Helper()
	sheet := decode[view.MailAttachSheet](t, f.h.do(get("/app/mail/attach?"+q.Encode(), f.sess)))
	if len(sheet.Preview) != 1 {
		t.Fatalf("%v: preview = %+v", q, sheet.Preview)
	}
	href := actionHref(t, sheet.Actions, "mail.attach.add")
	if rec := f.h.do(postForm(href, f.sess, url.Values{})); rec.Code != 303 {
		t.Fatalf("attach %v: %d %s", q, rec.Code, rec.Body)
	}
	return sheet
}

// parts are the attachments of a letter by file name.
func parts(t *testing.T, l mailtest.Letter) (string, map[string]string) {
	t.Helper()
	msg, err := mail.ReadMessage(strings.NewReader(l.Data))
	if err != nil {
		t.Fatal(err)
	}
	_, params, _ := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	mr := multipart.NewReader(msg.Body, params["boundary"])
	files := map[string]string{}
	var text string
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(p)
		if p.FileName() == "" {
			text = string(data)
			continue
		}
		if p.Header.Get("Content-Transfer-Encoding") == "base64" {
			data, _ = base64.StdEncoding.DecodeString(strings.ReplaceAll(string(data), "\r\n", ""))
		}
		files[p.FileName()] = string(data)
	}
	return text, files
}

// S-26.2: what goes as a file and what as a link is on the sheet before the
// letter goes; the letter carries the files, the links under the text, the
// name as signature; the journal says to whom and what kinds, not the text.
func TestLetterCarriesFilesAndLinks(t *testing.T) {
	f, relay := letterFixture(t)

	sheet := decode[view.MailSheet](t, f.h.do(get("/app/mail/compose?q=ada", f.sess)))
	var suggested []string
	for _, r := range sheet.Suggest {
		suggested = append(suggested, string(r.Address)+" "+string(r.Label)+" "+string(r.Name))
	}
	if strings.Join(suggested, "|") != "ada@example.com work Ada Lovelace|ada@home.example home Ada Lovelace" || len(sheet.NotAsked) != 0 {
		t.Errorf("suggest = %v, not asked = %v", suggested, sheet.NotAsked)
	}
	if sheet.ReplyTo != "" || !strings.Contains(string(sheet.ReplyNote), "no reply address") || sheet.From != "owner (via Carrel) <noreply@carrel.example>" {
		t.Errorf("from %q reply %q note %q", sheet.From, sheet.ReplyTo, sheet.ReplyNote)
	}

	rec := f.h.do(postForm("/app/mail/compose?op=attach&what=contact", f.sess, url.Values{
		"to": {"ann@example.org"}, "subject": {"Секретная тема"}, "body": {"Секретный текст"}}))
	if rec.Code != 303 || !strings.HasPrefix(rec.Header().Get("Location"), "/app/mail/attach?what=contact") {
		t.Fatalf("attach: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	list := decode[view.MailAttachSheet](t, f.h.do(get("/app/mail/attach?what=contact", f.sess)))
	if len(list.Rows) != 1 || list.Rows[0].Title != "Ada Lovelace" || len(list.Rail.Sections) != 1 {
		t.Fatalf("attach sheet = %+v", list)
	}
	card := attachOne(t, f, url.Values{"what": {"contact"}, "a": {f.id}, "c": {searchBook}, "uid": {"ada"}})
	if card.Preview[0].AsLink || card.Preview[0].Format != "vcf" {
		t.Errorf("card = %+v", card.Preview[0])
	}
	small := attachOne(t, f, url.Values{"what": {"file"}, "a": {f.id}, "c": {filesDocs}, "file": {"small.txt"}})
	big := attachOne(t, f, url.Values{"what": {"file"}, "a": {f.id}, "c": {filesDocs}, "file": {"big.bin"}})
	folder := attachOne(t, f, url.Values{"what": {"folder"}, "a": {f.id}, "c": {filesDocs}, "f": {"share"}})
	if small.Preview[0].AsLink || !big.Preview[0].AsLink || !folder.Preview[0].AsLink {
		t.Errorf("as link: small %v big %v folder %v", small.Preview[0].AsLink, big.Preview[0].AsLink, folder.Preview[0].AsLink)
	}

	sheet = decode[view.MailSheet](t, f.h.do(get("/app/mail/compose", f.sess)))
	if len(sheet.Attach) != 4 || sheet.Subject != "Секретная тема" || len(sheet.To) != 1 {
		t.Fatalf("sheet = %+v", sheet)
	}
	if len(f.h.users.PublishedLinks(f.sess.AccountID())) != 0 {
		t.Fatal("a link was made before the letter went")
	}
	rec = f.h.do(postForm(actionHref(t, sheet.Actions, "mail.send"), f.sess, url.Values{"to": {"ann@example.org"}, "subject": {"Секретная тема"}, "body": {"Секретный текст"}}))
	if rec.Code != 303 || !strings.Contains(rec.Header().Get("Location"), "sent=1") {
		t.Fatalf("send: %d %s", rec.Code, rec.Body)
	}

	letters := relay.Letters()
	if len(letters) != 1 || letters[0].To[0] != "ann@example.org" || letters[0].From != "noreply@carrel.example" {
		t.Fatalf("letters = %+v", letters)
	}
	body, files := parts(t, letters[0])
	if !strings.Contains(files["ada-lovelace.vcf"], "Ada Lovelace") || files["small.txt"] != "hello" || len(files) != 2 {
		t.Errorf("files = %v", files)
	}
	if strings.Count(body, "http://example.com/p/") != 2 || !strings.Contains(body, "owner") || !strings.Contains(body, "Do not reply") {
		t.Errorf("body:\n%s", body)
	}
	if n := len(f.h.users.PublishedLinks(f.sess.AccountID())); n != 2 {
		t.Errorf("links = %d", n)
	}
	snap := f.h.st.Snapshot()
	var journal string
	for _, e := range snap.Audit {
		if e.What == "mail.send" {
			journal = e.Detail
		}
	}
	if !strings.Contains(journal, "to ann@example.org") || !strings.Contains(journal, "contact ×1") || !strings.Contains(journal, "file link ×2") {
		t.Errorf("journal = %q", journal)
	}
	for _, e := range snap.Audit {
		if strings.Contains(e.Detail, "Секрет") {
			t.Fatalf("the journal holds the letter: %+v", e)
		}
	}
	if sheet := decode[view.MailSheet](t, f.h.do(get("/app/mail/compose", f.sess))); len(sheet.Attach) != 0 || sheet.Subject != "" {
		t.Errorf("the letter stayed after it went: %+v", sheet)
	}
}

// C-12: recipients and letters a day have ceilings, and the relay's refusal
// gives the day's letter back.
func TestLetterCeilings(t *testing.T) {
	f, relay := letterFixture(t)
	f.h.cfg.MailMaxPerDay = 1
	f.h.cfg.MailMaxRecipients = 2
	send := func(to ...string) view.MailSheet {
		rec := f.h.do(postForm("/app/mail/compose?op=send", f.sess, url.Values{"to": to, "subject": {"x"}, "body": {"x"}}))
		if rec.Code == 303 {
			return view.MailSheet{}
		}
		return decode[view.MailSheet](t, rec)
	}
	if sheet := send("a@example.org", "b@example.org", "c@example.org"); !strings.Contains(string(sheet.Limits), "2 recipients at most") {
		t.Errorf("three recipients: %q", sheet.Limits)
	}
	relay.Refuse = "nobody@example.org"
	if sheet := send("nobody@example.org"); !strings.Contains(string(sheet.Limits), "did not take") {
		t.Errorf("refused: %q", sheet.Limits)
	}
	if sheet := send("a@example.org"); sheet.Limits != "" {
		t.Fatalf("the refused letter was counted: %q", sheet.Limits)
	}
	sheet := send("a@example.org")
	if !strings.Contains(string(sheet.Limits), "as many letters today") {
		t.Errorf("second letter: %q", sheet.Limits)
	}
	for _, a := range sheet.Actions {
		if a.ID == "mail.send" && a.Enabled {
			t.Error("Send is offered with the day spent")
		}
	}
	if n := len(relay.Letters()); n != 1 {
		t.Errorf("letters = %d", n)
	}
}

// A book that did not answer is named, not silently missing.
func TestLetterNamesBooksThatDidNotAnswer(t *testing.T) {
	f, _ := letterFixture(t)
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		b.Connections = append(b.Connections, sealed.Connection{ID: "dead", Server: "http://127.0.0.1:1/dav/"})
		b.Collections = append(b.Collections, sealed.Collection{AccountID: "dead", Path: "/dav/office/", Name: "Office", Kinds: []view.Kind{view.KindContact}})
		b.Selected = append(b.Selected, sealed.Selection{Kind: view.KindContact, AccountID: "dead", Path: "/dav/office/"})
	})
	sheet := decode[view.MailSheet](t, f.h.do(get("/app/mail/compose?q=ada", f.sess)))
	if len(sheet.Suggest) != 2 || len(sheet.NotAsked) != 1 || !strings.Contains(string(sheet.NotAsked[0]), "Office") {
		t.Errorf("suggest %d, not asked %v", len(sheet.Suggest), sheet.NotAsked)
	}
}

// A link just made can be sent by letter from its sheet; it is the same
// link, not a second one.
func TestLinkJustMadeGoesByLetter(t *testing.T) {
	f, relay := letterFixture(t)
	made := decode[view.PublishSheet](t, f.h.do(postForm("/app/publish?"+url.Values{"k": {"contacts"}, "a": {f.id}, "c": {searchBook}, "uid": {"ada"}}.Encode(), f.sess, url.Values{})))
	rec := f.h.do(postForm(actionHref(t, made.Actions, "publish.mail"), f.sess, url.Values{}))
	if rec.Code != 303 {
		t.Fatalf("send by letter: %d", rec.Code)
	}
	sheet := decode[view.MailSheet](t, f.h.do(get("/app/mail/compose", f.sess)))
	if len(sheet.Attach) != 1 || !sheet.Attach[0].AsLink || sheet.Subject != "Ada Lovelace" {
		t.Fatalf("sheet = %+v", sheet)
	}
	if rec := f.h.do(postForm("/app/mail/compose?op=send", f.sess, url.Values{"to": {"ann@example.org"}, "subject": {"Ada"}, "body": {"See"}})); rec.Code != 303 {
		t.Fatalf("send: %d %s", rec.Code, rec.Body)
	}
	_, body := letterBody(t, relay.Letters()[0])
	if !strings.Contains(body, "http://example.com"+string(made.Link)) || len(f.h.users.PublishedLinks(f.sess.AccountID())) != 1 {
		t.Errorf("body:\n%s\nlink %s", body, made.Link)
	}
}

// Without mail on the instance there is no letter: the menu does not offer
// it and the sheet is not there.
func TestNoLetterWithoutMail(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	h.relayed()
	if rec := h.do(get("/app/mail/compose", alice)); rec.Code != 404 {
		t.Errorf("compose: %d", rec.Code)
	}
	page := decode[view.AccountPage](t, h.do(get("/app/settings/account", alice)))
	for _, m := range page.Top.New.Arrow {
		if m.ID == "new.letter" {
			t.Error("the menu offers a letter")
		}
	}
	h.cfg.MailEnabled = true
	h.rebuild()
	page = decode[view.AccountPage](t, h.do(get("/app/settings/account", alice)))
	n := len(page.Top.New.Arrow)
	if last, upload := page.Top.New.Arrow[n-1], page.Top.New.Arrow[n-2]; last.ID != "new.letter" || upload.ID != "new.upload" || !upload.Separator {
		t.Errorf("last items = %+v %+v", upload, last)
	}
}

// Owner's decision 19.09.2026: a link is kept whole in the person's blob, so
// Published links shows it, copies it and mails it later, from another
// session; the instance still keeps only its hash.
func TestOldLinkGoesByLetterFromPublishedLinks(t *testing.T) {
	f, relay := letterFixture(t)
	made := decode[view.PublishSheet](t, f.h.do(postForm("/app/publish?"+url.Values{"k": {"contacts"}, "a": {f.id}, "c": {searchBook}, "uid": {"ada"}}.Encode(), f.sess, url.Values{})))
	snap := f.h.st.Snapshot()
	if len(snap.Published) != 1 || strings.Contains(string(snap.Published[0].Hash), strings.TrimPrefix(string(made.Link), "/p/")) {
		t.Fatalf("published = %+v", snap.Published)
	}

	later := f.h.login("owner", "password123")
	page := decode[view.PublishedLinksPage](t, f.h.do(get("/app/files/published", later)))
	if len(page.Rows) != 1 || page.Rows[0].Link != made.Link || actionHref(t, page.Rows[0].Actions, "publish.copy") != string(made.Link) {
		t.Fatalf("rows = %+v", page.Rows)
	}
	if rec := f.h.do(postForm(actionHref(t, page.Rows[0].Actions, "publish.mail"), later, url.Values{})); rec.Code != 303 {
		t.Fatalf("send this link: %d", rec.Code)
	}
	if rec := f.h.do(postForm("/app/mail/compose?op=send", later, url.Values{"to": {"ann@example.org"}, "subject": {"Ada"}, "body": {"See"}})); rec.Code != 303 {
		t.Fatalf("send: %d %s", rec.Code, rec.Body)
	}
	_, body := letterBody(t, relay.Letters()[0])
	if !strings.Contains(body, "http://example.com"+string(made.Link)) || len(f.h.users.PublishedLinks(f.sess.AccountID())) != 1 {
		t.Errorf("body:\n%s", body)
	}
}

// «Calendar» with Dates takes its days from the sheet's own fields; both
// days are in the file, and a day outside is not.
func TestLetterCalendarRange(t *testing.T) {
	f := calendarFixture(t, func(h *harness) { h.cfg.MailEnabled = true },
		"in.ics", ics("BEGIN:VEVENT\nUID:in\nDTSTAMP:20260101T000000Z\nDTSTART:20261005T100000Z\nDTEND:20261005T110000Z\nSUMMARY:Inside\nEND:VEVENT"),
		"out.ics", ics("BEGIN:VEVENT\nUID:out\nDTSTAMP:20260101T000000Z\nDTSTART:20261020T100000Z\nDTEND:20261020T110000Z\nSUMMARY:Outside\nEND:VEVENT"))
	relay := f.h.relayed()
	q := url.Values{"what": {"calendar"}, "a": {f.id}, "c": {f.col}, "span": {"dates"}, "from": {"2026-10-01"}, "to": {"2026-10-10"}}
	sheet := attachOne(t, f, q)
	if sheet.Range == nil || formFields(*sheet.Range).Get("from") != "2026-10-01" || formFields(*sheet.Range).Get("to") != "2026-10-10" {
		t.Fatalf("range form = %+v", sheet.Range)
	}
	if rec := f.h.do(postForm("/app/mail/compose?op=send", f.sess, url.Values{"to": {"ann@example.org"}, "subject": {"October"}, "body": {"x"}})); rec.Code != 303 {
		t.Fatalf("send: %d %s", rec.Code, rec.Body)
	}
	_, files := parts(t, relay.Letters()[0])
	var ics string
	for _, v := range files {
		ics = v
	}
	if !strings.Contains(ics, "Inside") || strings.Contains(ics, "Outside") {
		t.Errorf("calendar:\n%s", ics)
	}
}

// «Calendar» opens on the whole calendar (owner's decision 24.09.2026,
// s-mail-10): no days to fill in, and every event goes into the file. The
// days and their Show appear only once Dates is chosen.
func TestLetterWholeCalendar(t *testing.T) {
	f := calendarFixture(t, func(h *harness) { h.cfg.MailEnabled = true },
		"in.ics", ics("BEGIN:VEVENT\nUID:in\nDTSTAMP:20260101T000000Z\nDTSTART:20261005T100000Z\nDTEND:20261005T110000Z\nSUMMARY:Inside\nEND:VEVENT"),
		"far.ics", ics("BEGIN:VEVENT\nUID:far\nDTSTAMP:20260101T000000Z\nDTSTART:20300101T100000Z\nDTEND:20300101T110000Z\nSUMMARY:Far away\nEND:VEVENT"))
	relay := f.h.relayed()
	sheet := attachOne(t, f, url.Values{"what": {"calendar"}, "a": {f.id}, "c": {f.col}})
	if sheet.Range == nil {
		t.Fatal("no choice of what goes in")
	}
	fields := formFields(*sheet.Range)
	if fields.Get("span") != "whole" || fields.Has("from") || fields.Has("to") || len(sheet.Range.Actions) != 0 {
		t.Errorf("whole calendar form = %+v", sheet.Range)
	}
	if !strings.Contains(string(sheet.Preview[0].Title), "all events") {
		t.Errorf("preview = %+v", sheet.Preview[0])
	}
	dates := decode[view.MailAttachSheet](t, f.h.do(get("/app/mail/attach?"+url.Values{"what": {"calendar"}, "a": {f.id}, "c": {f.col}, "span": {"dates"}}.Encode(), f.sess)))
	if df := formFields(*dates.Range); df.Get("span") != "dates" || df.Get("from") == "" || df.Get("to") == "" || len(dates.Range.Actions) != 1 {
		t.Errorf("dates form = %+v", dates.Range)
	}
	if rec := f.h.do(postForm("/app/mail/compose?op=send", f.sess, url.Values{"to": {"ann@example.org"}, "subject": {"All"}, "body": {"x"}})); rec.Code != 303 {
		t.Fatalf("send: %d %s", rec.Code, rec.Body)
	}
	_, files := parts(t, relay.Letters()[0])
	var got string
	for _, v := range files {
		got = v
	}
	if !strings.Contains(got, "Inside") || !strings.Contains(got, "Far away") {
		t.Errorf("calendar:\n%s", got)
	}
}

// A note goes as Markdown or as its source .ics: one choice, one Attach, and
// the preview names the choice (owner's decision 24.09.2026, s-mail-5).
func TestLetterNoteFormat(t *testing.T) {
	f := calendarFixture(t, func(h *harness) { h.cfg.MailEnabled = true },
		"n.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260101T000000Z\nDTSTART;VALUE=DATE:20260110\nSUMMARY:Plans\nDESCRIPTION:Body\nEND:VJOURNAL"))
	relay := f.h.relayed()
	base := url.Values{"what": {"note"}, "a": {f.id}, "c": {f.col}, "uid": {"n1"}}
	sheet := decode[view.MailAttachSheet](t, f.h.do(get("/app/mail/attach?"+base.Encode(), f.sess)))
	if sheet.Format == nil || formFields(*sheet.Format).Get("format") != "md" {
		t.Fatalf("format form = %+v", sheet.Format)
	}
	var attaches int
	for _, a := range sheet.Actions {
		if a.ID == "mail.attach.add" {
			attaches++
		}
	}
	if attaches != 1 || sheet.Preview[0].Format != "md" {
		t.Errorf("actions %+v, preview %+v", sheet.Actions, sheet.Preview)
	}
	base.Set("format", "ics")
	sheet = attachOne(t, f, base)
	if sheet.Preview[0].Format != "ics" || formFields(*sheet.Format).Get("format") != "ics" {
		t.Errorf("source chosen: preview %+v, form %+v", sheet.Preview, sheet.Format)
	}
	if rec := f.h.do(postForm("/app/mail/compose?op=send", f.sess, url.Values{"to": {"ann@example.org"}, "subject": {"Note"}, "body": {"x"}})); rec.Code != 303 {
		t.Fatalf("send: %d %s", rec.Code, rec.Body)
	}
	_, files := parts(t, relay.Letters()[0])
	if _, ok := files["plans.ics"]; !ok {
		t.Errorf("files = %v", files)
	}
}

// A recipient's label is the label the card shows (S-11): a TYPE that is no
// label — INTERNET, the vCard 3 default of every address — is not one, and
// «WORK,INTERNET» is work.
func TestRecipientLabelIsTheCardsLabel(t *testing.T) {
	f, _ := letterFixture(t)
	f.srv.Seed(searchBook+"grace.vcf", "BEGIN:VCARD\nVERSION:3.0\nUID:grace\nFN:Grace Hopper\n"+
		"EMAIL;TYPE=INTERNET:grace@example.org\nEMAIL;TYPE=WORK,INTERNET:grace@navy.example\nEND:VCARD\n")
	sheet := decode[view.MailSheet](t, f.h.do(get("/app/mail/compose?q=grace", f.sess)))
	got := map[string]string{}
	for _, r := range sheet.Suggest {
		got[string(r.Address)] = string(r.Label)
	}
	if got["grace@example.org"] != "" || got["grace@navy.example"] != "work" {
		t.Errorf("labels = %v", got)
	}
}
