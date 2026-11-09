package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// Typecase's note 122: the public pages — what the guest sees, the views
// without frames — and the calendar's filter.

const (
	pubTasks = davtest.CalendarHome + "tasks/"
	pubNotes = davtest.CalendarHome + "notes/"
)

// pub122 is a book, a calendar with a weekly meeting and a plain event, a
// task list and a notebook on one server.
func pub122(t *testing.T) davFixture {
	t.Helper()
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work", SupportedComponentSet: []string{"VEVENT"}})
	srv.AddCalendar(caldav.Calendar{Path: pubTasks, Name: "Tasks", SupportedComponentSet: []string{"VTODO"}})
	srv.AddCalendar(caldav.Calendar{Path: pubNotes, Name: "Notes", SupportedComponentSet: []string{"VJOURNAL"}})
	srv.Seed(searchBook+"ada.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nORG:Analytical Engines\nTITLE:Countess\n"+
		"TEL;TYPE=cell:+44 20 7946 0000\nEMAIL;TYPE=work:ada@example.com\nNOTE:Writes on Tuesdays.\nEND:VCARD\n")
	srv.Seed(searchWork+"sync.ics", ics("BEGIN:VEVENT\nUID:sync\nDTSTAMP:20260801T000000Z\nDTSTART:20260810T100000Z\nDTEND:20260810T110000Z\n"+
		"RRULE:FREQ=WEEKLY;UNTIL=20261228T100000Z\nSUMMARY:Sync on the release\nLOCATION:Room 2\nSTATUS:CONFIRMED\n"+
		"DESCRIPTION:Blockers and the freeze.\nORGANIZER;CN=Maria:mailto:maria@example.com\n"+
		"ATTENDEE;CN=Anna;PARTSTAT=ACCEPTED:mailto:anna@example.com\nATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:v@example.org\nEND:VEVENT"))
	srv.Seed(searchWork+"docs.ics", ics("BEGIN:VEVENT\nUID:docs\nDTSTAMP:20260801T000000Z\nDTSTART:20260810T180000Z\nDTEND:20260810T183000Z\nSUMMARY:Pick up the papers\nEND:VEVENT"))
	srv.Seed(pubTasks+"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260801T000000Z\nSUMMARY:Send the papers\nDUE;VALUE=DATE:20260807\n"+
		"LOCATION:The office\nSTATUS:IN-PROCESS\nPERCENT-COMPLETE:40\nDESCRIPTION:Passport and the form.\nEND:VTODO"))
	srv.Seed(pubTasks+"t2.ics", ics("BEGIN:VTODO\nUID:t2\nDTSTAMP:20260801T000000Z\nSUMMARY:Renew the insurance\nDUE;VALUE=DATE:20260803\nSTATUS:COMPLETED\nPERCENT-COMPLETE:100\nEND:VTODO"))
	srv.Seed(pubNotes+"n1.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260801T000000Z\nSUMMARY:Notes of the meeting\nDTSTART;VALUE=DATE:20260810\nDESCRIPTION:Freeze on Wednesday.\nEND:VJOURNAL"))
	srv.Seed(pubNotes+"n2.ics", ics("BEGIN:VJOURNAL\nUID:n2\nDTSTAMP:20260801T000000Z\nSUMMARY:What to read\nDTSTART;VALUE=DATE:20260708\nDESCRIPTION:Kleppmann.\nEND:VJOURNAL"))
	return connectFixture(t, srv, searchBook, nil)
}

// pubWhole is a link to the whole calendar, its other ticks as given.
func pubWhole(t *testing.T, f davFixture, ticks ...string) string {
	t.Helper()
	sheet := decode[view.PublishSheet](t, f.h.do(get(f.publishHref("calendar", searchWork, ""), f.sess)))
	form := url.Values{"how": {"whole"}, "title": {"on"}, "lives": {"never"}}
	for _, name := range ticks {
		form.Set(name, "on")
	}
	got := decode[view.PublishSheet](t, f.h.do(postForm(actionHref(t, sheet.Actions, "publish.create"), f.sess, form)))
	if !strings.HasPrefix(string(got.Link), "/p/") {
		t.Fatalf("no link: %+v", got)
	}
	return string(got.Link)
}

// frameSub is the line under a frame's title.
var frameSub = regexp.MustCompile(`(?s)<div class="tc-head__sub[^"]*">(.*?)</div>`)

// frameFoot is a public page's foot.
var frameFoot = regexp.MustCompile(`<div class="tc-main__foot">(.*?)</div>`)

// frameEmptyLine is the one line of an empty view.
var frameEmptyLine = regexp.MustCompile(`<div class="tc-empty"><b>(.*?)</b></div>`)

// s-publish-24, -25, -6: the whole calendar steps by weeks — ← Today →, the
// arrows saying where they go —, its rows open their events in the link and
// keep «repeats weekly»; without the download no strip and no button, the
// foot one phrase; a week without events is one line, without the filter.
// With the download the strip subscribes: webcal:// and «Open in calendar
// app», what it is muted, the file at the strip's own name.
func TestPublicCalendarIsTheFrames(t *testing.T) {
	whole, _, _ := strings.Cut(mockupFrame(t, "s-publish-24"), "<figcaption")
	sub := frameOne(t, frameSub, whole, "the days under the title")[1]
	foot := frameOne(t, frameFoot, whole, "foot")[1]
	arrows := regexp.MustCompile(`(?s)<a class="tc-btn" href="[^"]*" title="([^"]*)">(?:<svg.*?</svg>)?</a><a class="tc-btn" href="[^"]*">([^<]*)</a><a class="tc-btn" href="[^"]*" title="([^"]*)">(?:<svg.*?</svg>)?</a>`).FindStringSubmatch(whole)
	filter := frameOne(t, regexp.MustCompile(`name="filter" placeholder="([^"]*)"`), whole, "filter")[1]
	empty := frameOne(t, frameEmptyLine, mockupFrame(t, "s-publish-25"), "empty week")[1]
	if arrows == nil || strings.Contains(whole, "tc-main__strip") || strings.Contains(whole, "Download") {
		t.Fatal("s-publish-24 changed: read it again")
	}
	f := pub122(t)
	link := pubWhole(t, f)
	page := publicPage(t, f, link+"?from=2026-08-10")
	if runsText(page.Lede) != sub || runsText(page.Foot) != foot || page.Strip != nil || len(page.Actions) != 0 {
		t.Errorf("head %q, foot %q, strip %+v, actions %+v; the frame %q, %q, none", runsText(page.Lede), runsText(page.Foot), page.Strip, page.Actions, sub, foot)
	}
	if page.Range == nil || len(page.Range.Presets) != 3 || page.Range.Presets[0].Label != view.Text(arrows[1]) ||
		page.Range.Presets[1].Label != view.Text(arrows[2]) || page.Range.Presets[2].Label != view.Text(arrows[3]) {
		t.Errorf("range = %+v, the frame %q", page.Range, arrows[1:])
	}
	if page.Filter == nil || formRow(*page.Filter, "filter") == nil || string(formRow(*page.Filter, "filter").Label) != filter || page.Filter.Method != "GET" {
		t.Errorf("filter = %+v, the frame «%s» by q", page.Filter, filter)
	}
	repeats := false
	for _, d := range page.Days {
		for _, r := range d.Rows {
			if !strings.HasPrefix(r.Href, link+"?uid=") {
				t.Errorf("row %q opens %q, want its event in the link", r.Summary, r.Href)
			}
			for _, b := range r.Badges {
				repeats = repeats || b.Label == "repeats weekly"
			}
		}
	}
	if !repeats {
		t.Errorf("days = %+v: the series lost «repeats weekly»", page.Days)
	}
	quiet := publicPage(t, f, link+"?from=2026-07-06")
	if string(quiet.Empty) != empty || quiet.Filter != nil || len(quiet.Days) != 0 {
		t.Errorf("an empty week: %q, filter %+v; the frame %q, none", quiet.Empty, quiet.Filter, empty)
	}

	// The download allowed: the strip of s-publish-6.
	sixth := mockupFrame(t, "s-publish-6")
	label := frameOne(t, regexp.MustCompile(`<span class="tc-dim--muted tc-dim--t12">([^<]*)</span>`), sixth, "strip label")[1]
	note := frameOne(t, regexp.MustCompile(`<span class="tc-dim tc-dim--t11">([^<]*)</span>`), sixth, "strip note")[1]
	if !strings.Contains(sixth, `href="webcal://`) || !strings.Contains(sixth, ">Open in calendar app</a>") {
		t.Fatal("s-publish-6 opens no calendar app: read it again")
	}
	sub6 := pubWhole(t, f, "download")
	strip := publicPage(t, f, sub6).Strip
	if strip == nil || string(strip.Label) != label || string(strip.Note) != note || !strings.HasPrefix(strip.Open, "webcal://") || string(strip.Address) != strip.Open {
		t.Fatalf("strip = %+v, the frame %q, %q", strip, label, note)
	}
	file := "/" + strip.Open[strings.Index(strip.Open, "/p/")+1:]
	if rec := f.h.do(anon(file)); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "UID:sync") {
		t.Errorf("the strip's address %s: %d", file, rec.Code)
	}
	if rec := f.h.do(anon(sub6 + "/other.ics")); rec.Code != http.StatusNotFound {
		t.Errorf("another name answers %d", rec.Code)
	}
}

// s-publish-16, -27, -26: one record — when and where under the title, not
// fields; an event's Repeats and Status, its description and its people by
// name, without links; a task's due day, its status with its share; a
// contact's initials, its organisation and title under the name, the fields
// ticked below.
func TestPublicRecordIsTheFrames(t *testing.T) {
	event := mockupFrame(t, "s-publish-16")
	labels := regexp.MustCompile(`<dt>([^<]*)</dt>`).FindAllStringSubmatch(event, -1)
	if len(labels) != 2 || labels[0][1] != "Repeats" || labels[1][1] != "Status" || !strings.Contains(event, `<div class="tc-rubric">Attendees`) {
		t.Fatalf("s-publish-16 fields %q: read it again", labels)
	}
	f := pub122(t)
	ev := publicPage(t, f, publishWith(t, f, f.publishHref("calendar", searchWork, "sync"), "title", "location", "description", "attendees"))
	if !regexp.MustCompile(`^Monday, 10 August 2026, \d\d:\d\d – \d\d:\d\d · Room 2$`).MatchString(runsText(ev.Lede)) {
		t.Errorf("event lede %q, the frame «day, time – time · place»", runsText(ev.Lede))
	}
	var got []string
	for _, fl := range ev.Fields {
		got = append(got, string(fl.Label))
	}
	if strings.Join(got, " ") != "Repeats Status" || ev.Body == "" || len(ev.Attendees) != 3 {
		t.Errorf("event fields %q, body %q, people %+v", got, ev.Body, ev.Attendees)
	}
	for _, a := range ev.Attendees {
		if a.Href != "" || a.Colour != "" {
			t.Errorf("a person leads into the owner's books: %+v", a)
		}
	}

	task := mockupFrame(t, "s-publish-27")
	if !regexp.MustCompile(`<div class="tc-head__sub">Due [A-Z][a-z]+day, \d+ [A-Z][a-z]+ \d{4}</div>`).MatchString(task) || !strings.Contains(task, `<dt>Status</dt><dd>Needs action</dd>`) {
		t.Fatal("s-publish-27 changed: read it again")
	}
	tk := publicPage(t, f, publishWith(t, f, f.publishHref("tasks", pubTasks, "t1"), "title", "location", "description"))
	if runsText(tk.Lede) != "Due Friday, 7 August 2026 · The office" || len(tk.Fields) != 1 || runsText(tk.Fields[0].Runs) != "In process 40%" {
		t.Errorf("task lede %q, fields %+v", runsText(tk.Lede), tk.Fields)
	}

	contact := mockupFrame(t, "s-publish-26")
	if !strings.Contains(contact, `<span class="tc-avatar--photo is-ph">`) || !strings.Contains(contact, `<div class="tc-head__sub">Издательство «Слово» · Выпускающий редактор</div>`) {
		t.Fatal("s-publish-26 changed: read it again")
	}
	ada := publicPage(t, f, publishWith(t, f, f.publishHref("contacts", searchBook, "ada"), "phones", "emails", "note"))
	if ada.Avatar == nil || ada.Avatar.Initials == "" || runsText(ada.Lede) != "Analytical Engines · Countess" {
		t.Errorf("contact head %+v, %q", ada.Avatar, runsText(ada.Lede))
	}
	for _, fl := range ada.Fields {
		if strings.Contains(string(fl.Value), "Analytical") || fl.Label == "Title" {
			t.Errorf("the head again in the fields: %+v", fl)
		}
	}
}

// s-publish-28, -29, -35, -7: a task list under «Open» and «Done» with their
// counts, a notebook by month, the newest first, each row opening its
// record in the link; a book's rows too; an empty book says so, without a
// filter.
func TestPublicListsAreTheFrames(t *testing.T) {
	tasks := mockupFrame(t, "s-publish-28")
	rubrics := regexp.MustCompile(`<span class="tc-rubric">([^<]*)</span><span class="tc-num">(\d+)</span>`).FindAllStringSubmatch(tasks, -1)
	if len(rubrics) != 2 || rubrics[0][1] != "Open" || rubrics[1][1] != "Done" {
		t.Fatalf("s-publish-28 rubrics %q: read it again", rubrics)
	}
	f := pub122(t)
	tl := publicPage(t, f, publishWith(t, f, f.publishHref("tasks", pubTasks, ""), "title"))
	if len(tl.Tasks) != 2 || string(tl.Tasks[0].Rubric) != rubrics[0][1] || string(tl.Tasks[1].Rubric) != rubrics[1][1] || tl.Tasks[0].Count != 1 || runsText(tl.Lede) != "2 tasks" {
		t.Fatalf("tasks = %+v, lede %q", tl.Tasks, runsText(tl.Lede))
	}
	if r := tl.Tasks[0].Rows[0]; !strings.Contains(r.Href, "?uid=t1") || !r.Overdue {
		t.Errorf("the open task's row = %+v", r)
	}

	notes := mockupFrame(t, "s-publish-29")
	month := frameOne(t, regexp.MustCompile(`<span class="tc-rubric">([^<]*)</span>`), notes, "month")[1]
	if !regexp.MustCompile(`^[A-Z][a-z]+ \d{4}$`).MatchString(month) {
		t.Fatalf("s-publish-29 rubric %q: read it again", month)
	}
	nb := publicPage(t, f, publishWith(t, f, f.publishHref("notes", pubNotes, ""), "title"))
	if len(nb.Notes) != 2 || nb.Notes[0].Rubric != "August 2026" || nb.Notes[1].Rubric != "July 2026" || !strings.Contains(nb.Notes[0].Rows[0].Href, "?uid=n1") {
		t.Errorf("notebook = %+v", nb.Notes)
	}
	opened := publicPage(t, f, nb.Notes[0].Rows[0].Href)
	if opened.Note == nil || opened.Note.Title != "Notes of the meeting" {
		t.Errorf("the note's row opens %+v", opened)
	}

	book := publicPage(t, f, publishWith(t, f, f.publishHref("contacts", searchBook, ""), "emails"))
	if len(book.Contacts) != 1 || !strings.Contains(book.Contacts[0].Rows[0].Href, "?uid=ada") || runsText(book.Lede) != "1 contact" {
		t.Errorf("book = %+v, lede %q", book.Contacts, runsText(book.Lede))
	}
	if card := publicPage(t, f, book.Contacts[0].Rows[0].Href); card.Title != "Ada Lovelace" || card.Avatar == nil {
		t.Errorf("the contact's row opens %+v", card)
	}
	if none := publicPage(t, f, book.Contacts[0].Rows[0].Href+"x").Notices; noticeTitled(none, "No longer there.") == nil {
		t.Errorf("a row of nothing: %+v", none)
	}
	filtered := publicPage(t, f, publishWith(t, f, f.publishHref("contacts", searchBook, ""))+"?filter=nobody")
	if len(filtered.Contacts) != 0 || filtered.Filter == nil {
		t.Errorf("filtered = %+v", filtered.Contacts)
	}

	empty := frameOne(t, frameEmptyLine, mockupFrame(t, "s-publish-35"), "empty book")[1]
	f.srv.Unseed(searchBook + "ada.vcf")
	gone := publicPage(t, f, publishWith(t, f, f.publishHref("contacts", searchBook, "")))
	if string(gone.Empty) != empty || gone.Filter != nil || runsText(gone.Lede) != "0 contacts" {
		t.Errorf("an empty book: %q, filter %+v, %q; the frame %q", gone.Empty, gone.Filter, runsText(gone.Lede), empty)
	}
}

// s-publish-12, -36, -32: a folder counts items, its name opens it in the
// link and Download is its archive; «Filter this folder»; the drive's strip
// with «How to mount», a line for each system; an empty folder says so, no
// archive.
func TestPublicFolderIsTheFrames(t *testing.T) {
	frame := mockupFrame(t, "s-publish-12")
	total := frameOne(t, regexp.MustCompile(`<span class="tc-dim">· ([^<]*)</span>`), frame, "total")[1]
	filter := frameOne(t, regexp.MustCompile(`name="filter" placeholder="([^"]*)"`), frame, "filter")[1]
	label := frameOne(t, regexp.MustCompile(`<span class="tc-dim--muted tc-dim--t12">([^<]*)</span>`), frame, "strip label")[1]
	note := frameOne(t, regexp.MustCompile(`<span class="tc-dim tc-dim--t11">([^<]*)</span>`), frame, "strip note")[1]
	var mount []string
	for _, m := range regexp.MustCompile(`<div class="tc-menu__text"><b>([^<]*)</b>([^<]*)</div>`).FindAllStringSubmatch(frame, -1) {
		mount = append(mount, m[1]+": "+m[2])
	}
	if !regexp.MustCompile(`^\d+ items · [\d.]+ [KMG]?B$`).MatchString(total) || len(mount) != 3 {
		t.Fatalf("s-publish-12 total %q, mount %q: read it again", total, mount)
	}
	f := pubDocs(t)
	link := publishFile(t, f, "share", true)
	page := publicPage(t, f, link)
	if page.Folder == nil || !regexp.MustCompile(`^\d+ items · [\d.]+ [KMG]?B$`).MatchString(string(page.Folder.Total)) {
		t.Fatalf("folder = %+v", page.Folder)
	}
	for _, r := range page.Folder.Rows {
		switch {
		case r.IsDir && (!strings.Contains(r.Open, "?in=") || !strings.Contains(r.Href, "/download?in=") || !strings.HasPrefix(string(r.Type), "folder")):
			t.Errorf("a folder's row %+v", r)
		case !r.IsDir && (!strings.Contains(r.Open, "?file=") || !strings.Contains(r.Href, "/file/")):
			t.Errorf("a file's row %+v", r)
		}
	}
	if page.Filter == nil || string(formRow(*page.Filter, "filter").Label) != filter {
		t.Errorf("filter = %+v, the frame %q", page.Filter, filter)
	}
	var got []string
	if page.Strip != nil {
		for _, m := range page.Strip.Mount {
			got = append(got, string(m.Title)+": "+string(m.Text))
		}
	}
	if page.Strip == nil || string(page.Strip.Label) != label || string(page.Strip.Note) != note || strings.Join(got, "\n") != strings.Join(mount, "\n") {
		t.Errorf("strip = %+v, the frame %q, %q, %q", page.Strip, label, note, mount)
	}

	empty := frameOne(t, frameEmptyLine, mockupFrame(t, "s-publish-36"), "empty folder")[1]
	f.srv.SeedFolder(filesDocs + "drafts/")
	none := publicPage(t, f, publishFile(t, f, "drafts", true))
	if string(none.Empty) != empty || len(none.Actions) != 0 || none.Filter != nil || none.Strip == nil || none.Folder.Total != "0 items" {
		t.Errorf("an empty folder: %q, actions %+v, filter %+v, total %q; the frame %q", none.Empty, none.Actions, none.Filter, none.Folder.Total, empty)
	}
}

// s-publish-11, -34, -33: a file without a preview says so in one line —
// «A browser cannot show this file.», a text too large «Too large to show
// on the page.» —; a PDF is the browser's, in the page's frame.
func TestPublicFileLinesAreTheFrames(t *testing.T) {
	note := regexp.MustCompile(`<div class="tc-main__stage-note">([^<]*)</div>`)
	cannot := frameOne(t, note, mockupFrame(t, "s-publish-11"), "line")[1]
	large := frameOne(t, note, mockupFrame(t, "s-publish-34"), "line")[1]
	f := pubDocs(t)
	f.srv.SeedFile(filesDocs+"share/cover.psd", strings.Repeat("x", 3000))
	f.srv.SeedFile(filesDocs+"share/log.txt", strings.Repeat("a", 256<<10+1))
	f.srv.SeedFile(filesDocs+"share/bill.pdf", "%PDF-1.4")
	if p := publicPage(t, f, publishFile(t, f, "share/cover.psd", false)); p.File == nil || string(p.File.Note) != cannot {
		t.Errorf("a file not shown: %+v, the frame %q", p.File, cannot)
	}
	if p := publicPage(t, f, publishFile(t, f, "share/log.txt", false)); p.File == nil || string(p.File.Note) != large {
		t.Errorf("a large text: %+v, the frame %q", p.File, large)
	}
	if p := publicPage(t, f, publishFile(t, f, "share/bill.pdf", false)); p.File == nil || p.File.Preview != view.PreviewPDF || p.File.Note != "" || !strings.HasSuffix(p.File.Href, "?inline=1") {
		t.Errorf("a PDF: %+v", p.File)
	}
}

// s-publish-37, -38: the source did not answer — the frame's two sentences,
// the foot stays —; what is gone — its two, no foot.
func TestPublicTroubleIsTheFrames(t *testing.T) {
	msg := regexp.MustCompile(`<div class="tc-msg[^"]*"><b>([^<]*)</b> ([^<]*)</div>`)
	down := frameOne(t, msg, mockupFrame(t, "s-publish-37"), "message")
	gone := frameOne(t, msg, mockupFrame(t, "s-publish-38"), "message")
	if frameFoot.MatchString(mockupFrame(t, "s-publish-38")) || !frameFoot.MatchString(mockupFrame(t, "s-publish-37")) {
		t.Fatal("the feet of s-publish-37, -38 changed: read them again")
	}
	f := pub122(t)
	link := publishWith(t, f, f.publishHref("contacts", searchBook, ""))
	f.srv.Refuse("REPORT", http.StatusServiceUnavailable, "down")
	p := publicPage(t, f, link)
	if n := noticeTitled(p.Notices, down[1]); n == nil || string(n.Body) != down[2] || n.Role != view.NoticeAlert || len(p.Foot) == 0 {
		t.Errorf("down = %+v, foot %q; the frame %q", p.Notices, runsText(p.Foot), down[1:])
	}
	g := pub122(t)
	one := publishWith(t, g, g.publishHref("contacts", searchBook, "ada"))
	g.srv.Unseed(searchBook + "ada.vcf")
	p = publicPage(t, g, one)
	if n := noticeTitled(p.Notices, gone[1]); n == nil || string(n.Body) != gone[2] || len(p.Foot) != 0 {
		t.Errorf("gone = %+v, foot %q; the frame %q", p.Notices, runsText(p.Foot), gone[1:])
	}
}

// s-agenda-31, s-publish-39: a word narrows what the week shows, for the
// owner and the guest — «… · 1 of 3 events», ← Today → with the word
// (Typecase's note 185).
func TestCalendarFilterIsTheFrames(t *testing.T) {
	owner := mockupFrame(t, "s-agenda-31")
	if !strings.Contains(mockupFrame(t, "s-agenda-17"), `<input class="tc-q" type="search" name="filter" placeholder="Filter by word"`) ||
		!strings.Contains(owner, " · 1 of 3 events</div>") || !strings.Contains(owner, `title="Previous week"`) {
		t.Fatal("s-agenda-17, -31 changed: read them again")
	}
	f := pub122(t)
	cal := "/app/calendar/" + f.id + "/" + url.PathEscape(searchWork)
	week := decode[view.AgendaPage](t, f.h.do(get(cal+"?from=2026-08-10", f.sess)))
	if week.Bar.FilterForm == nil || formRow(*week.Bar.FilterForm, "filter") == nil {
		t.Fatalf("the week's filter = %+v", week.Bar.FilterForm)
	}
	found := decode[view.AgendaPage](t, f.h.do(get(cal+"?from=2026-08-10&filter=sync", f.sess)))
	if found.Bar.Range == nil || len(found.Days) != 1 || len(found.Days[0].Rows) != 1 || !strings.HasSuffix(runsText(found.Subtitle), " · 1 of 2 events") {
		t.Errorf("found = %+v, subtitle %q", found.Days, runsText(found.Subtitle))
	}

	guest := mockupFrame(t, "s-publish-39")
	if !strings.Contains(guest, `<div class="tc-head__sub">10 – 16 August 2026 · 1 of 3 events</div>`) || !strings.Contains(guest, "?from=2026-08-03&amp;filter=") {
		t.Fatal("s-publish-39 changed: read it again")
	}
	link := pubWhole(t, f)
	p := publicPage(t, f, link+"?from=2026-08-10&filter=papers")
	if p.Range == nil || runsText(p.Lede) != "10 – 16 August 2026 · 1 of 2 events" || len(p.Days) != 1 || !strings.HasPrefix(p.Days[0].Rows[0].Href, link+"?uid=") {
		t.Errorf("the guest's found = %+v, lede %q", p.Days, runsText(p.Lede))
	}
	if back := p.Range.Presets[0].Href; !strings.Contains(back, "filter=papers") {
		t.Errorf("← leads to %q without the word", back)
	}
}

// s-publish-10, -8: the width of a text read on the page is an address —
// «Full width», «Reading width» at ?width=reading —, no script needed.
func TestPublicWidthIsTheFrame(t *testing.T) {
	m := regexp.MustCompile(`<a class="is-on tc-seg__opt" href="(/p/[^"?]*)">([^<]*)</a><a class="tc-seg__opt" href="(/p/[^"]*)">([^<]*)</a>`).FindStringSubmatch(mockupFrame(t, "s-publish-10"))
	if m == nil || m[3] != m[1]+"?width=reading" {
		t.Fatalf("s-publish-10 width = %q: read it again", m)
	}
	f := pubDocs(t)
	f.srv.SeedFile(filesDocs+"share/plan.md", "# Plan\n\nWords.")
	link := publishFile(t, f, "share/plan.md", false)
	for _, c := range []struct {
		addr string
		on   int
	}{{link, 0}, {link + "?width=reading", 1}} {
		w := publicPage(t, f, c.addr).Width
		if len(w) != 2 || w[0].Label != view.Text(m[2]) || w[1].Label != view.Text(m[4]) || w[0].Href != link || w[1].Href != link+"?width=reading" || !w[c.on].On || w[1-c.on].On {
			t.Errorf("%s: width = %+v, the frame %q · %q", c.addr, w, m[2], m[4])
		}
	}
}
