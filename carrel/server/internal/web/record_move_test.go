package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

const movedCard = "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada\nX-CUSTOM:keep\nEND:VCARD\n"

// moveFixture is S-18's ground: People (with Ada), Friends and a read-only
// Archive on one server, and Work on a second account.
type moveFixture struct {
	davFixture
	other   *davtest.Server
	otherID string
}

var (
	peopleBook  = davtest.AddressBookHome + "people/"
	friendsBook = davtest.AddressBookHome + "friends/"
	archiveBook = davtest.AddressBookHome + "archive/"
	workBook    = davtest.AddressBookHome + "work/"
)

func newMoveFixture(t *testing.T) moveFixture {
	t.Helper()
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	srv.AddAddressBook(carddav.AddressBook{Path: peopleBook, Name: "People"})
	srv.AddAddressBook(carddav.AddressBook{Path: friendsBook, Name: "Friends"})
	srv.AddAddressBook(carddav.AddressBook{Path: archiveBook, Name: "Archive", ReadOnly: true})
	srv.Seed(peopleBook+"ada.vcf", movedCard)
	f := connectFixture(t, srv, peopleBook, nil)

	other := davtest.New(t)
	other.Login, other.Password = "bob", "other"
	other.AddAddressBook(carddav.AddressBook{Path: workBook, Name: "Work"})
	return moveFixture{davFixture: f, other: other, otherID: addAccount(t, f, other)}
}

// addAccount connects a second server for the same person and returns its id.
func addAccount(t *testing.T, f davFixture, srv *davtest.Server) string {
	t.Helper()
	rec := f.h.do(postForm("/app/settings/connections", f.sess, url.Values{
		"server": {srv.URL}, "login": {srv.Login}, "password": {srv.Password},
	}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("connect the second account: %d %s", rec.Code, rec.Body.String())
	}
	key, _ := f.sess.DataKey()
	blob, _ := f.h.users.Catalog(f.sess.AccountID(), key)
	if len(blob.Connections) != 2 {
		t.Fatalf("connections = %d, want 2", len(blob.Connections))
	}
	return blob.Connections[1].ID
}

func (f moveFixture) record(account, book, uid string) string {
	return "/app/contacts/" + account + "/" + url.PathEscape(book) + "/" + url.PathEscape(uid)
}

func moveOption(t *testing.T, sheet view.MoveSheet, label string) view.TargetOption {
	t.Helper()
	for _, o := range sheet.Target.Options {
		if string(o.Label) == label {
			return o
		}
	}
	t.Fatalf("no option %q in %+v", label, sheet.Target.Options)
	return view.TargetOption{}
}

func confirmOf(t *testing.T, sheet view.MoveSheet) view.Action {
	t.Helper()
	if len(sheet.Actions) == 0 {
		t.Fatalf("sheet has no actions: %+v", sheet)
	}
	return sheet.Actions[0]
}

// S-18: every collection of the kind is listed, the current and read-only
// ones closed with the reason; another account is named before the start.
func TestMoveSheetSaysBeforeItStarts(t *testing.T) {
	f := newMoveFixture(t)
	ada := f.record(f.id, peopleBook, "ada")

	page := decode[view.ContactPage](t, f.h.do(get(ada, f.sess)))
	moveHref := actionHref(t, page.Actions, "record.move")
	if copyHref := actionHref(t, page.Actions, "record.copy"); copyHref != ada+"/move?copy=1" {
		t.Errorf("copy = %q", copyHref)
	}

	sheet := decode[view.MoveSheet](t, f.h.do(get(moveHref, f.sess)))
	if len(sheet.Target.Options) != 4 || sheet.Copy || sheet.Kind != view.KindContact {
		t.Fatalf("sheet = %+v", sheet)
	}
	for label, reason := range map[string]string{"People": "Already here", "Archive": "Read-only", "Friends": "", "Work": ""} {
		o := moveOption(t, sheet, label)
		if o.ReadOnly != (reason != "") || string(o.Reason) != reason {
			t.Errorf("%s: readOnly %v reason %q", label, o.ReadOnly, o.Reason)
		}
	}
	if c := confirmOf(t, sheet); c.Enabled || c.Reason == "" || sheet.Target.Chosen != "" {
		t.Errorf("nothing chosen, confirm = %+v chosen %q", c, sheet.Target.Chosen)
	}

	friends := moveOption(t, sheet, "Friends").ID
	chosen := decode[view.MoveSheet](t, f.h.do(get(friends, f.sess)))
	if c := confirmOf(t, chosen); !c.Enabled || c.Method != "POST" || chosen.Target.Chosen != friends || chosen.Warning != "" {
		t.Errorf("same account: confirm %+v chosen %q warning %q", c, chosen.Target.Chosen, chosen.Warning)
	}

	work := decode[view.MoveSheet](t, f.h.do(get(moveOption(t, sheet, "Work").ID, f.sess)))
	if work.Warning == "" || !confirmOf(t, work).Enabled {
		t.Errorf("another account must be named before the move: %+v", work)
	}
	copySheet := decode[view.MoveSheet](t, f.h.do(get(moveOption(t, sheet, "Work").ID+"&copy=1", f.sess)))
	if !copySheet.Copy || copySheet.Warning != "" || confirmOf(t, copySheet).ID != "record.copy.confirm" {
		t.Errorf("copy to another account: %+v", copySheet)
	}

	frag := f.h.do(fragment(get(moveHref, f.sess), "sheet"))
	if frag.Code != http.StatusOK || !strings.Contains(frag.Body.String(), `"target":"sheet"`) {
		t.Errorf("fragment: %d %s", frag.Code, frag.Body.String())
	}
}

func postConfirm(t *testing.T, f moveFixture, sheetHref string) *http.Request {
	t.Helper()
	sheet := decode[view.MoveSheet](t, f.h.do(get(sheetHref, f.sess)))
	c := confirmOf(t, sheet)
	if !c.Enabled {
		t.Fatalf("confirm disabled: %+v", c)
	}
	return postForm(c.Href, f.sess, url.Values{})
}

func sheetFor(f moveFixture, from, uid, account, book string, copying bool) string {
	q := url.Values{"to": {account}, "into": {book}}
	if copying {
		q.Set("copy", "1")
	}
	return f.record(f.id, from, uid) + "/move?" + q.Encode()
}

// On one account a move is one MOVE: no body travels, every byte stays.
func TestMoveOnOneAccountIsAMove(t *testing.T) {
	f := newMoveFixture(t)
	before := body(t, f.davFixture, "ada.vcf")
	resp := f.h.do(postConfirm(t, f, sheetFor(f, peopleBook, "ada", f.id, friendsBook, false)))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != f.record(f.id, friendsBook, "ada") {
		t.Fatalf("move: %d %q %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	if n, puts := f.srv.CountRequests("MOVE", ""), f.srv.CountRequests(http.MethodPut, ""); n != 1 || puts != 0 {
		t.Errorf("MOVE %d, PUT %d; want one MOVE and no PUT", n, puts)
	}
	if got, _, ok := f.srv.Object(friendsBook + "ada.vcf"); !ok || got != before {
		t.Errorf("moved body = %q", got)
	}
	if _, _, ok := f.srv.Object(peopleBook + "ada.vcf"); ok {
		t.Error("the original is still in People")
	}
	page := decode[view.ContactPage](t, f.h.do(get(f.record(f.id, friendsBook, "ada"), f.sess)))
	if page.Name != "Ada" {
		t.Errorf("record at the target = %+v", page)
	}
}

// To another account: create there, read back, compare, then delete here.
func TestMoveToAnotherAccountCopiesThenDeletes(t *testing.T) {
	f := newMoveFixture(t)
	resp := f.h.do(postConfirm(t, f, sheetFor(f, peopleBook, "ada", f.otherID, workBook, false)))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != f.record(f.otherID, workBook, "ada") {
		t.Fatalf("move: %d %q %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	copied, _, ok := f.other.Object(workBook + "ada.vcf")
	if !ok {
		t.Fatalf("no copy on the other account: %v", f.other.Paths(workBook))
	}
	mustContain(t, copied, "UID:ada", "FN:Ada", "X-CUSTOM:keep")
	if _, _, ok := f.srv.Object(peopleBook + "ada.vcf"); ok {
		t.Error("the original is still in People")
	}
	if f.srv.CountRequests("MOVE", "") != 0 {
		t.Error("MOVE between accounts")
	}
}

// A copy that lost something is not a reason to delete the original.
func TestMoveKeepsTheOriginalWhenTheCopyLoses(t *testing.T) {
	f := newMoveFixture(t)
	f.other.Drop = []string{"X-CUSTOM"}
	before := body(t, f.davFixture, "ada.vcf")
	page := decode[view.ContactPage](t, f.h.do(postConfirm(t, f, sheetFor(f, peopleBook, "ada", f.otherID, workBook, false))))
	if !hasNotice(page.Notices, "Copied, and the original kept") || !strings.Contains(string(page.Notices[0].Body), "X-CUSTOM") {
		t.Fatalf("notices = %+v", page.Notices)
	}
	if actionHref(t, page.Notices[0].Actions, "record.open") != f.record(f.otherID, workBook, "ada") {
		t.Error("the notice does not open the copy")
	}
	if body(t, f.davFixture, "ada.vcf") != before {
		t.Error("the original changed")
	}
	if _, _, ok := f.other.Object(workBook + "ada.vcf"); !ok {
		t.Error("the copy is gone")
	}
}

// A record changed after it was read stays, and so does the copy.
func TestMoveKeepsAChangedOriginal(t *testing.T) {
	f := newMoveFixture(t)
	req := postConfirm(t, f, sheetFor(f, peopleBook, "ada", f.otherID, workBook, false))
	f.srv.Refuse(http.MethodDelete, http.StatusPreconditionFailed, "changed")
	page := decode[view.ContactPage](t, f.h.do(req))
	if !hasNotice(page.Notices, "Copied, but the original changed, so it stays") {
		t.Fatalf("notices = %+v", page.Notices)
	}
	if _, _, ok := f.srv.Object(peopleBook + "ada.vcf"); !ok {
		t.Error("the original was deleted")
	}
}

// A copy leaves the original, keeps the UID, and never MOVEs.
func TestCopyKeepsTheOriginalAndTheUID(t *testing.T) {
	f := newMoveFixture(t)
	before := body(t, f.davFixture, "ada.vcf")
	resp := f.h.do(postConfirm(t, f, sheetFor(f, peopleBook, "ada", f.id, friendsBook, true)))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != f.record(f.id, friendsBook, "ada") {
		t.Fatalf("copy: %d %q %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	if body(t, f.davFixture, "ada.vcf") != before {
		t.Error("the original changed")
	}
	copied, _, ok := f.srv.Object(friendsBook + "ada.vcf")
	if !ok {
		t.Fatal("no copy in Friends")
	}
	mustContain(t, copied, "UID:ada", "X-CUSTOM:keep")
	if f.srv.CountRequests("MOVE", "") != 0 || f.srv.CountRequests(http.MethodDelete, "") != 0 {
		t.Error("a copy moved or deleted")
	}
}

// A taken file name becomes {uid}-2; a taken UID stops before any write.
func TestMoveTakenNameAndTakenUID(t *testing.T) {
	t.Run("name", func(t *testing.T) {
		for _, copying := range []bool{false, true} {
			f := newMoveFixture(t)
			f.srv.Seed(friendsBook+"ada.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:someone-else\nFN:Else\nEND:VCARD\n")
			other, _, _ := f.srv.Object(friendsBook + "ada.vcf")
			resp := f.h.do(postConfirm(t, f, sheetFor(f, peopleBook, "ada", f.id, friendsBook, copying)))
			if resp.Code != http.StatusSeeOther {
				t.Fatalf("copy=%v: %d %s", copying, resp.Code, resp.Body.String())
			}
			if got, _, ok := f.srv.Object(friendsBook + "ada.vcf"); !ok || got != other {
				t.Errorf("copy=%v: the taken name was overwritten: %q", copying, got)
			}
			moved, _, ok := f.srv.Object(friendsBook + "ada-2.vcf")
			if !ok {
				t.Fatalf("copy=%v: paths %v", copying, f.srv.Paths(friendsBook))
			}
			mustContain(t, moved, "UID:ada")
		}
	})
	t.Run("uid", func(t *testing.T) {
		f := newMoveFixture(t)
		f.other.Seed(workBook+"elsewhere.vcf", movedCard)
		page := decode[view.ContactPage](t, f.h.do(postConfirm(t, f, sheetFor(f, peopleBook, "ada", f.otherID, workBook, false))))
		if !hasNotice(page.Notices, "Nothing was moved") {
			t.Fatalf("notices = %+v", page.Notices)
		}
		if f.other.CountRequests(http.MethodPut, "") != 0 || f.srv.CountRequests(http.MethodDelete, "") != 0 {
			t.Error("written despite the taken UID")
		}
	})
}

// A read-only record can be copied, not moved; a read-only target takes
// nothing, whatever the request says.
func TestReadOnlyAndMove(t *testing.T) {
	f := newMoveFixture(t)
	f.srv.Seed(archiveBook+"old.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:old\nFN:Old\nEND:VCARD\n")
	old := f.record(f.id, archiveBook, "old")

	card := decode[view.ContactPage](t, f.h.do(get(old, f.sess)))
	if len(card.Actions) != 1 || card.Actions[0].ID != "record.copy" {
		t.Errorf("read-only actions = %+v", card.Actions)
	}
	for _, req := range []*http.Request{
		get(old+"/move", f.sess),
		postForm(old+"/move", f.sess, url.Values{"to": {f.id}, "into": {peopleBook}}),
	} {
		if resp := f.h.do(req); resp.Code != http.StatusNotFound {
			t.Errorf("%s move from read-only: %d, want 404", req.Method, resp.Code)
		}
	}
	resp := f.h.do(postConfirm(t, f, sheetFor(f, archiveBook, "old", f.id, peopleBook, true)))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("copy from read-only: %d %s", resp.Code, resp.Body.String())
	}
	if _, _, ok := f.srv.Object(archiveBook + "old.vcf"); !ok {
		t.Error("the read-only original is gone")
	}

	ada := f.record(f.id, peopleBook, "ada")
	page := decode[view.ContactPage](t, f.h.do(postForm(ada+"/move", f.sess, url.Values{"to": {f.id}, "into": {archiveBook}})))
	if !hasNotice(page.Notices, "Nothing was moved") {
		t.Errorf("into read-only: %+v", page.Notices)
	}
	if _, _, ok := f.srv.Object(peopleBook + "ada.vcf"); !ok || f.srv.CountRequests("MOVE", "") != 0 {
		t.Error("moved into a read-only collection")
	}
	sheet := decode[view.MoveSheet](t, f.h.do(get(sheetFor(f, peopleBook, "ada", f.id, archiveBook, false), f.sess)))
	if c := confirmOf(t, sheet); c.Enabled {
		t.Errorf("confirm into read-only = %+v", c)
	}
}

// S-18, S-7: an event travels with its time zone and its overridden
// instances, not rebuilt from what the page shows.
func TestMovedEventTakesItsZoneAndInstances(t *testing.T) {
	event := ics("BEGIN:VTIMEZONE\nTZID:Europe/Moscow\nBEGIN:STANDARD\nDTSTART:19700101T000000\nTZOFFSETFROM:+0300\nTZOFFSETTO:+0300\nEND:STANDARD\nEND:VTIMEZONE\n" +
		"BEGIN:VEVENT\nUID:standup\nDTSTAMP:20260101T000000Z\nDTSTART;TZID=Europe/Moscow:20260302T100000\nRRULE:FREQ=DAILY;COUNT=5\nSUMMARY:Standup\nEND:VEVENT\n" +
		"BEGIN:VEVENT\nUID:standup\nDTSTAMP:20260101T000000Z\nRECURRENCE-ID;TZID=Europe/Moscow:20260303T100000\nDTSTART;TZID=Europe/Moscow:20260303T110000\nSUMMARY:Standup, later\nEND:VEVENT")
	f := calendarFixture(t, nil, "standup.ics", event)
	other := davtest.New(t)
	other.Login, other.Password = "bob", "other"
	team := davtest.CalendarHome + "team/"
	other.AddCalendar(caldav.Calendar{Path: team, Name: "Team"})
	otherID := addAccount(t, f, other)

	record := f.path("calendar") + "/standup"
	q := url.Values{"to": {otherID}, "into": {team}}
	sheet := decode[view.MoveSheet](t, f.h.do(get(record+"/move?"+q.Encode(), f.sess)))
	if sheet.Kind != view.KindEvent || sheet.Warning == "" {
		t.Fatalf("sheet = %+v", sheet)
	}
	resp := f.h.do(postForm(confirmOf(t, sheet).Href, f.sess, url.Values{}))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("move: %d %s", resp.Code, resp.Body.String())
	}
	moved, _, ok := other.Object(team + "standup.ics")
	if !ok {
		t.Fatalf("paths %v", other.Paths(team))
	}
	mustContain(t, moved, "BEGIN:VTIMEZONE", "TZID:Europe/Moscow", "RRULE:FREQ=DAILY;COUNT=5", "RECURRENCE-ID;TZID=Europe/Moscow:20260303T100000", "SUMMARY:Standup, later")
	if _, _, ok := f.srv.Object(f.col + "standup.ics"); ok {
		t.Error("the original is still there")
	}
}
