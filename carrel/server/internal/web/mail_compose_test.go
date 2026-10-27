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
	// Attach sends the sheet's form: what is chosen in its address, what to
	// send it as in its fields.
	href := actionHref(t, sheet.Actions, "mail.attach.add")
	if rec := f.h.do(postForm(href, f.sess, formFields(sheet.Form))); rec.Code != 303 {
		t.Fatalf("attach %v: %d %s", q, rec.Code, rec.Body)
	}
	return sheet
}

// letterValue is what stands in a field of the letter's form.
func letterValue(sheet view.MailSheet, name string) string {
	if r := formRow(sheet.Form, name); r != nil {
		return string(r.Value)
	}
	return ""
}

// letterTo are the recipients of the letter's form, a tag each.
func letterTo(sheet view.MailSheet) []view.Option {
	if r := formRow(sheet.Form, "to"); r != nil {
		return r.Options
	}
	return nil
}

// letterHint is the line under To: what the last press did, how many, that
// all see all (s-mail-1).
func letterHint(sheet view.MailSheet) string {
	if r := formRow(sheet.Form, "to"); r != nil {
		return string(r.Hint)
	}
	return ""
}

// letterToValues are the addresses of the recipients, as the form sends them.
func letterToValues(sheet view.MailSheet) []string {
	var out []string
	for _, o := range letterTo(sheet) {
		out = append(out, o.Value)
	}
	return out
}

func letterHintRuns(sheet view.MailSheet) view.Runs {
	if r := formRow(sheet.Form, "to"); r != nil {
		return r.HintRuns
	}
	return nil
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

	sheet := decode[view.MailSheet](t, f.h.do(postForm("/app/mail/send?op=add", f.sess, url.Values{"to.add": {"ada"}})))
	var suggested []string
	for _, r := range sheet.Suggest {
		suggested = append(suggested, string(r.Address)+" "+string(r.Label)+" "+string(r.Name))
	}
	if strings.Join(suggested, "|") != "ada@example.com work Ada Lovelace|ada@home.example home Ada Lovelace" || len(sheet.NotAsked) != 0 {
		t.Errorf("suggest = %v, not asked = %v", suggested, sheet.NotAsked)
	}
	if sheet.ReplyTo != "" || !strings.Contains(saidRuns(sheet.ReplyNote), "no reply address") || sheet.From != "noreply@carrel.example" || sheet.FromName != "owner (via Carrel)" {
		t.Errorf("from %q reply %q note %q", sheet.From, sheet.ReplyTo, sheet.ReplyNote)
	}

	rec := f.h.do(postForm("/app/mail/send?op=attach&what=contact", f.sess, url.Values{
		"to": {"ann@example.org"}, "subject": {"Секретная тема"}, "text": {"Секретный текст"}}))
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
	small := attachOne(t, f, url.Values{"what": {"file"}, "a": {f.id}, "c": {filesDocs}, "path": {"small.txt"}})
	big := attachOne(t, f, url.Values{"what": {"file"}, "a": {f.id}, "c": {filesDocs}, "path": {"big.bin"}})
	folder := attachOne(t, f, url.Values{"what": {"folder"}, "a": {f.id}, "c": {filesDocs}, "dir": {"share"}})
	if small.Preview[0].AsLink || !big.Preview[0].AsLink || !folder.Preview[0].AsLink {
		t.Errorf("as link: small %v big %v folder %v", small.Preview[0].AsLink, big.Preview[0].AsLink, folder.Preview[0].AsLink)
	}

	sheet = decode[view.MailSheet](t, f.h.do(get("/app/mail/compose", f.sess)))
	if len(sheet.Attach) != 4 || letterValue(sheet, "subject") != "Секретная тема" || len(letterTo(sheet)) != 1 {
		t.Fatalf("sheet = %+v", sheet)
	}
	if len(f.h.users.PublishedLinks(f.sess.AccountID())) != 0 {
		t.Fatal("a link was made before the letter went")
	}
	rec = f.h.do(postForm(actionHref(t, sheet.Form.Actions, "mail.send"), f.sess, url.Values{"to": {"ann@example.org"}, "subject": {"Секретная тема"}, "text": {"Секретный текст"}}))
	if rec.Code != 303 || rec.Header().Get("Location") != "/app/contacts" {
		t.Fatalf("send: %d %s %s", rec.Code, rec.Header().Get("Location"), rec.Body)
	}
	// The sheet closed to the screen under it, which says once that the
	// letter went (s-mail-16).
	if page := f.h.do(get("/app/contacts", f.sess)); !strings.Contains(page.Body.String(), "The letter went to 1 recipient.") {
		t.Error("the screen under the letter does not say it went")
	}
	if page := f.h.do(get("/app/contacts", f.sess)); strings.Contains(page.Body.String(), "The letter went to") {
		t.Error("the line that the letter went stays past the next page")
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
	// The name under «-- » (s-mail-7): the links before it, a mail program
	// greying what follows; the line that nobody hears a reply after it
	// (s-mail-8), with no second «-- ».
	text, sig, _ := strings.Cut(strings.ReplaceAll(body, "\r\n", "\n"), "\n-- \n")
	if !strings.HasPrefix(sig, "owner\n") || strings.Count(text, "http://example.com/p/") != 2 || !strings.Contains(sig, "Do not reply") || strings.Contains(sig, "-- ") {
		t.Errorf("text:\n%s\nsignature:\n%s", text, sig)
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
	if sheet := decode[view.MailSheet](t, f.h.do(get("/app/mail/compose", f.sess))); len(sheet.Attach) != 0 || letterValue(sheet, "subject") != "" {
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
		rec := f.h.do(postForm("/app/mail/send?op=send", f.sess, url.Values{"to": to, "subject": {"x"}, "body": {"x"}}))
		if rec.Code == 303 {
			return view.MailSheet{}
		}
		return decode[view.MailSheet](t, rec)
	}
	if sheet := send("a@example.org", "b@example.org", "c@example.org"); !strings.Contains(letterHint(sheet), "2 recipients at most") {
		t.Errorf("three recipients: %q", letterHint(sheet))
	}
	relay.Refuse = "nobody@example.org"
	// Refused, the letter stays as it was, the relay's words first under To
	// in alarm (s-mail-17).
	if sheet := send("nobody@example.org"); !strings.HasPrefix(letterHint(sheet), "The letter did not go: the relay answered «550 no such mailbox».") ||
		len(letterHintRuns(sheet)) != 2 || letterHintRuns(sheet)[0].Mark != view.RunDanger {
		t.Errorf("refused: %q %+v", letterHint(sheet), letterHintRuns(sheet))
	}
	if sheet := send("a@example.org"); letterHint(sheet) != "" {
		t.Fatalf("the refused letter was counted: %q", letterHint(sheet))
	}
	sheet := send("a@example.org")
	if !strings.Contains(letterHint(sheet), "as many letters today") {
		t.Errorf("second letter: %q", letterHint(sheet))
	}
	for _, a := range sheet.Form.Actions {
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
	sheet := decode[view.MailSheet](t, f.h.do(postForm("/app/mail/send?op=add", f.sess, url.Values{"to.add": {"ada"}})))
	if len(sheet.Suggest) != 2 || len(sheet.NotAsked) != 1 || !strings.Contains(string(sheet.NotAsked[0]), "Office") {
		t.Errorf("suggest %d, not asked %v", len(sheet.Suggest), sheet.NotAsked)
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

// «Calendar» with Dates takes its days from the sheet's own fields; both
// days are in the file, and a day outside is not.
func TestLetterCalendarRange(t *testing.T) {
	f := calendarFixture(t, func(h *harness) { h.cfg.MailEnabled = true },
		"in.ics", ics("BEGIN:VEVENT\nUID:in\nDTSTAMP:20260101T000000Z\nDTSTART:20261005T100000Z\nDTEND:20261005T110000Z\nSUMMARY:Inside\nEND:VEVENT"),
		"out.ics", ics("BEGIN:VEVENT\nUID:out\nDTSTAMP:20260101T000000Z\nDTSTART:20261020T100000Z\nDTEND:20261020T110000Z\nSUMMARY:Outside\nEND:VEVENT"))
	relay := f.h.relayed()
	q := url.Values{"what": {"calendar"}, "a": {f.id}, "c": {f.col}, "span": {"dates"}, "from": {"2026-10-01"}, "to": {"2026-10-10"}}
	sheet := attachOne(t, f, q)
	if fields := formFields(sheet.Form); fields.Get("span") != "dates" || fields.Get("from") != "2026-10-01" || fields.Get("to") != "2026-10-10" {
		t.Fatalf("the sheet's form = %+v", sheet.Form)
	}
	if rec := f.h.do(postForm("/app/mail/send?op=send", f.sess, url.Values{"to": {"ann@example.org"}, "subject": {"October"}, "text": {"x"}})); rec.Code != 303 {
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
// s-mail-10): every event goes into the file. The days stand always, from
// today to the end of the month, off while the whole calendar is chosen
// (С27); there is no Show — Attach sends the choice.
func TestLetterWholeCalendar(t *testing.T) {
	f := calendarFixture(t, func(h *harness) { h.cfg.MailEnabled = true },
		"in.ics", ics("BEGIN:VEVENT\nUID:in\nDTSTAMP:20260101T000000Z\nDTSTART:20261005T100000Z\nDTEND:20261005T110000Z\nSUMMARY:Inside\nEND:VEVENT"),
		"far.ics", ics("BEGIN:VEVENT\nUID:far\nDTSTAMP:20260101T000000Z\nDTSTART:20300101T100000Z\nDTEND:20300101T110000Z\nSUMMARY:Far away\nEND:VEVENT"))
	relay := f.h.relayed()
	sheet := attachOne(t, f, url.Values{"what": {"calendar"}, "a": {f.id}, "c": {f.col}})
	fields := formFields(sheet.Form)
	from, to := formRow(sheet.Form, "from"), formRow(sheet.Form, "to")
	span := formRow(sheet.Form, "span")
	if fields.Get("span") != "whole" || from == nil || to == nil || !from.ReadOnly || !to.ReadOnly || span == nil || span.Options[0].Disables != "from to" {
		t.Errorf("whole calendar form = %+v", sheet.Form)
	}
	if !strings.Contains(string(sheet.Preview[0].Title), "all events") {
		t.Errorf("preview = %+v", sheet.Preview[0])
	}
	dates := decode[view.MailAttachSheet](t, f.h.do(get("/app/mail/attach?"+url.Values{"what": {"calendar"}, "a": {f.id}, "c": {f.col}, "span": {"dates"}}.Encode(), f.sess)))
	if df := formFields(dates.Form); df.Get("span") != "dates" || df.Get("from") == "" || df.Get("to") == "" || formRow(dates.Form, "from").ReadOnly {
		t.Errorf("dates form = %+v", dates.Form)
	}
	if rec := f.h.do(postForm("/app/mail/send?op=send", f.sess, url.Values{"to": {"ann@example.org"}, "subject": {"All"}, "text": {"x"}})); rec.Code != 303 {
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
	if formFields(sheet.Form).Get("format") != "md" {
		t.Fatalf("format form = %+v", sheet.Form)
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
	if sheet.Preview[0].Format != "ics" || formFields(sheet.Form).Get("format") != "ics" {
		t.Errorf("source chosen: preview %+v, form %+v", sheet.Preview, sheet.Form)
	}
	if rec := f.h.do(postForm("/app/mail/send?op=send", f.sess, url.Values{"to": {"ann@example.org"}, "subject": {"Note"}, "text": {"x"}})); rec.Code != 303 {
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
	sheet := decode[view.MailSheet](t, f.h.do(postForm("/app/mail/send?op=add", f.sess, url.Values{"to.add": {"grace"}})))
	got := map[string]string{}
	for _, r := range sheet.Suggest {
		got[string(r.Address)] = string(r.Label)
	}
	if got["grace@example.org"] != "" || got["grace@navy.example"] != "work" {
		t.Errorf("labels = %v", got)
	}
}

// Пб38 (the owner's decision Б38): a link a letter makes gives the guest no
// more than the sheet of publishing it gives untouched — a calendar its
// times and titles, the .ics allowed; a contact its name and organisation,
// no .vcf. Before, it gave the whole of the record: attendees, addresses,
// descriptions.
func TestLetterLinksGiveWhatThePublishSheetGives(t *testing.T) {
	f, _ := letterFixture(t)
	f.srv.Seed(searchBook+"big.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:big\nFN:Big Card\nTEL:+1 555 0100\nEMAIL:big@example.com\nNOTE:"+strings.Repeat("n", 2000)+"\nEND:VCARD\n")
	card := attachOne(t, f, url.Values{"what": {"contact"}, "a": {f.id}, "c": {searchBook}, "uid": {"big"}})
	if !card.Preview[0].AsLink {
		t.Fatalf("the big card does not go as a link: %+v", card.Preview[0])
	}
	if rec := f.h.do(postForm("/app/mail/send?op=send", f.sess, url.Values{"to": {"ann@example.org"}, "subject": {"Card"}, "text": {"x"}})); rec.Code != 303 {
		t.Fatalf("send: %d %s", rec.Code, rec.Body)
	}
	links := f.h.users.PublishedLinks(f.sess.AccountID())
	if len(links) != 1 {
		t.Fatalf("links = %+v", links)
	}
	if got := links[0].Target; len(got.Shows) != 0 || got.HideOrganisation || !got.NoDownload {
		t.Errorf("the card's link = %+v, want the name and organisation, no .vcf", got)
	}

	cal := calendarFixture(t, func(h *harness) {
		h.cfg.MailEnabled = true
		h.cfg.MailMaxAttachBytes = 10
	}, "meet.ics", ics("BEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\nDTSTART:20261005T100000Z\nDTEND:20261005T110000Z\nSUMMARY:Meet\n"+
		"DESCRIPTION:Secret plan\nATTENDEE:mailto:ann@example.org\nEND:VEVENT"))
	cal.h.relayed()
	whole := attachOne(t, cal, url.Values{"what": {"calendar"}, "a": {cal.id}, "c": {cal.col}})
	if !whole.Preview[0].AsLink {
		t.Fatalf("the calendar does not go as a link: %+v", whole.Preview[0])
	}
	if rec := cal.h.do(postForm("/app/mail/send?op=send", cal.sess, url.Values{"to": {"ann@example.org"}, "subject": {"Cal"}, "text": {"x"}})); rec.Code != 303 {
		t.Fatalf("send: %d %s", rec.Code, rec.Body)
	}
	links = cal.h.users.PublishedLinks(cal.sess.AccountID())
	if len(links) != 1 {
		t.Fatalf("links = %+v", links)
	}
	if got := links[0].Target; strings.Join(got.Shows, ",") != "time,title" || !got.NoDownload {
		t.Errorf("the calendar's link = %+v, want the time and the title, no .ics — as the sheet stands", got)
	}
}

// Preview is the letter as the recipients get it, and makes nothing
// (s-mail-14); Cancel without text closes the letter, with text asks first
// (s-mail-15): Carrel keeps no drafts.
func TestLetterPreviewAndClose(t *testing.T) {
	f, relay := letterFixture(t)
	attachOne(t, f, url.Values{"what": {"file"}, "a": {f.id}, "c": {filesDocs}, "path": {"small.txt"}})
	attachOne(t, f, url.Values{"what": {"folder"}, "a": {f.id}, "c": {filesDocs}, "dir": {"share"}})
	typed := url.Values{"to": {"ann@example.org"}, "subject": {"Тема"}, "text": {"Текст"}}

	preview := decode[view.MailPreviewSheet](t, f.h.do(postForm("/app/mail/send?op=preview", f.sess, typed)))
	if saidRuns(preview.From) != "⟨strong:owner (via Carrel)⟩ ⟨mono:<noreply@carrel.example>⟩" || saidRuns(preview.To) != "ann@example.org" || preview.Subject != "Тема" {
		t.Errorf("headers: from %q to %q subject %q", saidRuns(preview.From), saidRuns(preview.To), preview.Subject)
	}
	if preview.Text != "Текст\n\n-- \nowner" || len(preview.ReplyTo) != 0 || !strings.Contains(string(preview.NoReply), "Do not reply") {
		t.Errorf("text %q reply %+v no reply %q", preview.Text, preview.ReplyTo, preview.NoReply)
	}
	if len(preview.Files) != 1 || preview.Files[0].Name != "small.txt" || len(preview.Links) != 1 ||
		!strings.HasPrefix(string(preview.Links[0].Title), "share, ") || preview.Links[0].Address != "example.com/p/…" {
		t.Errorf("files %+v links %+v", preview.Files, preview.Links)
	}
	if len(f.h.users.PublishedLinks(f.sess.AccountID())) != 0 || len(relay.Letters()) != 0 {
		t.Fatal("Preview made a link or sent the letter")
	}

	closed := decode[view.MailCloseSheet](t, f.h.do(postForm(actionHref(t, preview.Form.Actions, "mail.cancel"), f.sess, url.Values{})))
	if saidRuns(closed.Warning) != "Carrel keeps no drafts: ⟨strong:closing loses the text⟩." {
		t.Errorf("warning = %q", saidRuns(closed.Warning))
	}
	if keep := decode[view.MailSheet](t, f.h.do(get(actionHref(t, closed.Form.Actions, "mail.keep"), f.sess))); letterValue(keep, "text") != "Текст" || len(keep.Attach) != 2 {
		t.Errorf("Keep writing lost the letter: %+v", keep)
	}
	if rec := f.h.do(postForm(actionHref(t, closed.Form.Actions, "mail.discard"), f.sess, url.Values{})); rec.Code != 303 || rec.Header().Get("Location") != "/app/contacts" {
		t.Fatalf("discard: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if sheet := decode[view.MailSheet](t, f.h.do(get("/app/mail/compose", f.sess))); len(sheet.Attach) != 0 || letterValue(sheet, "subject") != "" {
		t.Errorf("the letter stayed after it was discarded: %+v", sheet)
	}

	attachOne(t, f, url.Values{"what": {"file"}, "a": {f.id}, "c": {filesDocs}, "path": {"small.txt"}})
	sheet := decode[view.MailSheet](t, f.h.do(get("/app/mail/compose", f.sess)))
	rec := f.h.do(postForm(actionHref(t, sheet.Form.Actions, "mail.cancel"), f.sess, url.Values{"to": {"ann@example.org"}, "subject": {"Тема"}, "text": {" "}}))
	if rec.Code != 303 || rec.Header().Get("Location") != "/app/contacts" {
		t.Fatalf("Cancel without text: %d %s", rec.Code, rec.Body)
	}
	if sheet := decode[view.MailSheet](t, f.h.do(get("/app/mail/compose", f.sess))); len(sheet.Attach) != 0 {
		t.Errorf("the letter stayed after Cancel: %+v", sheet)
	}
}

// The relay refusing some (s-mail-18): the letter goes to the rest, the
// sheet says who got it and who did not with the relay's words, and «Send
// again to the refused» is the same letter to them, with the links it made.
// Refusing all (s-mail-17), it makes no link.
func TestLetterToSomeOfThem(t *testing.T) {
	f, relay := letterFixture(t)
	attachOne(t, f, url.Values{"what": {"folder"}, "a": {f.id}, "c": {filesDocs}, "dir": {"share"}})
	relay.Refuse, relay.RefuseReply = "nobody@example.org", "550 5.1.1 Mailbox unavailable"

	sent := decode[view.MailSentSheet](t, f.h.do(postForm("/app/mail/send", f.sess,
		url.Values{"to": {"ann@example.org", "nobody@example.org"}, "subject": {"Тема"}, "text": {"Текст"}})))
	if sent.Notice != "The letter went, but not to everyone. Sending again goes only to the refused." || len(sent.Rows) != 2 ||
		sent.Rows[0].Said != "sent" || sent.Rows[0].Refused ||
		sent.Rows[1].Address != "nobody@example.org" || sent.Rows[1].Said != "refused: «550 5.1.1 Mailbox unavailable»" || !sent.Rows[1].Refused {
		t.Fatalf("sent = %+v", sent)
	}
	if letters := relay.Letters(); len(letters) != 1 || strings.Join(letters[0].To, ",") != "ann@example.org" {
		t.Fatalf("letters = %+v", letters)
	}
	links := f.h.users.PublishedLinks(f.sess.AccountID())
	if len(links) != 1 {
		t.Fatalf("links = %d", len(links))
	}

	again := decode[view.MailSheet](t, f.h.do(get(actionHref(t, sent.Actions, "mail.again"), f.sess)))
	if to := letterTo(again); len(to) != 1 || to[0].Value != "nobody@example.org" || letterValue(again, "subject") != "Тема" || len(again.Attach) != 1 {
		t.Fatalf("again = %+v", again)
	}
	relay.Refuse = ""
	if rec := f.h.do(postForm("/app/mail/send", f.sess, url.Values{"to": {"nobody@example.org"}, "subject": {"Тема"}, "text": {"Текст"}})); rec.Code != 303 {
		t.Fatalf("again: %d %s", rec.Code, rec.Body)
	}
	letters := relay.Letters()
	_, first, _ := strings.Cut(letters[0].Data, "\r\n\r\n")
	_, second, _ := strings.Cut(letters[len(letters)-1].Data, "\r\n\r\n")
	if len(letters) != 2 || len(f.h.users.PublishedLinks(f.sess.AccountID())) != 1 || first != second || !strings.Contains(first, "/p/") {
		t.Errorf("sent again, the letter differs or made another link:\n%s\n%s", first, second)
	}

	// Any other way in, it is a new letter.
	if sheet := decode[view.MailSheet](t, f.h.do(get("/app/mail/compose", f.sess))); len(sheet.Attach) != 0 {
		t.Errorf("the letter stayed: %+v", sheet)
	}
	attachOne(t, f, url.Values{"what": {"folder"}, "a": {f.id}, "c": {filesDocs}, "dir": {"share"}})
	relay.Refuse = "nobody@example.org"
	refused := decode[view.MailSheet](t, f.h.do(postForm("/app/mail/send", f.sess, url.Values{"to": {"nobody@example.org"}, "text": {"x"}})))
	if !strings.HasPrefix(letterHint(refused), "The letter did not go") || len(refused.Attach) != 1 {
		t.Errorf("refused = %q %+v", letterHint(refused), refused.Attach)
	}
	if n := len(f.h.users.PublishedLinks(f.sess.AccountID())); n != 1 {
		t.Errorf("a letter that did not go made a link: %d", n)
	}
}
