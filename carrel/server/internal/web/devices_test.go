package web_test

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// deviceFixture: one person, two servers — a calendar and a book on the
// first, a book on the second.
func deviceFixture(t *testing.T) (davFixture, string) {
	t.Helper()
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "s3cret&<pass>"
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	f := connectFixture(t, srv, searchBook, nil)
	other := davtest.New(t)
	other.Login, other.Password = "bob", "other"
	other.AddAddressBook(carddav.AddressBook{Path: workBook, Name: "Team"})
	return f, addAccount(t, f, other)
}

func devicesPage(t *testing.T, f davFixture, tab string) view.DevicesPage {
	t.Helper()
	return decode[view.DevicesPage](t, f.h.do(get("/app/settings/devices?tab="+tab, f.sess)))
}

func deviceAction(t *testing.T, actions []view.Action, id string) view.Action {
	t.Helper()
	for _, a := range actions {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("no action %q in %+v", id, actions)
	return view.Action{}
}

func unlock(t *testing.T, f davFixture) {
	t.Helper()
	rec := f.h.do(fragment(postForm("/app/settings/unlock", f.sess, url.Values{"password": {"password123"}}), "sheet"))
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "Password is wrong") {
		t.Fatalf("unlock: %d %s", rec.Code, rec.Body.String())
	}
}

func journaled(f davFixture, what string) []string {
	var out []string
	for _, e := range f.h.st.Snapshot().Audit {
		if e.What == what {
			out = append(out, e.Detail)
		}
	}
	return out
}

// Outside the unlock window the profile is not issued: the button opens the
// sheet, and a POST says so and issues nothing.
func TestProfileNeedsTheUnlockWindow(t *testing.T) {
	f, _ := deviceFixture(t)
	p := devicesPage(t, f, "apple")
	gen := deviceAction(t, p.Actions, "device.profile")
	// Why it asks is the line above the button (s-plan-3), not a reason
	// of a button that works (C-6.2).
	if gen.Method != "GET" || gen.Href != "/app/settings/unlock" || gen.Reason != "" || !strings.HasPrefix(string(p.Note), "Confirm your password first") {
		t.Errorf("locked generate = %+v, note %q", gen, p.Note)
	}
	if p.Warning.Role != view.NoticeAlert || !strings.Contains(string(p.Warning.Title), "passwords") {
		t.Errorf("warning = %+v", p.Warning)
	}
	rec := f.h.do(postForm("/app/settings/devices?tab=apple&op=profile", f.sess, url.Values{}))
	got := decode[view.DevicesPage](t, rec)
	if !hasNotice(got.Notices, "Confirm your password first") || len(journaled(f, "device.profile.link")) != 0 {
		t.Errorf("locked POST issued a link: %+v", got.Notices)
	}
}

// S-23: inside the window the link is issued and journaled with the
// connections; followed once it is the profile — built in memory, not
// cached, with every included account; followed again it is dead.
func TestProfileLinkOpensOnce(t *testing.T) {
	f, otherID := deviceFixture(t)
	unlock(t, f)
	got := decode[view.DevicesPage](t, f.h.do(postForm(deviceAction(t, devicesPage(t, f, "apple").Actions, "device.profile").Href, f.sess, url.Values{})))
	var link string
	for _, n := range got.Notices {
		if strings.HasPrefix(string(n.Title), "Link ready until") {
			link = deviceAction(t, n.Actions, "device.profile.download").Href
		}
	}
	if link == "" {
		t.Fatalf("no link: %+v", got.Notices)
	}
	if j := journaled(f, "device.profile.link"); len(j) != 1 || !strings.Contains(j[0], f.id) || !strings.Contains(j[0], otherID) {
		t.Errorf("journal = %v", j)
	}

	rec := f.h.do(get(link, f.sess))
	if rec.Code != http.StatusOK {
		t.Fatalf("profile: %d", rec.Code)
	}
	if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Content-Type") != "application/x-apple-aspen-config" {
		t.Errorf("headers = %v", rec.Header())
	}
	body := rec.Body.String()
	for _, want := range []string{"com.apple.caldav.account", "com.apple.carddav.account", "<string>alice</string>",
		"<string>s3cret&amp;&lt;pass&gt;</string>", "<string>bob</string>", "<string>other</string>"} {
		if !strings.Contains(body, want) {
			t.Errorf("profile lacks %q", want)
		}
	}
	if strings.Count(body, "com.apple.carddav.account") != 2 || strings.Count(body, "com.apple.caldav.account") != 1 {
		t.Errorf("payloads:\n%s", body)
	}
	if j := journaled(f, "device.profile.download"); len(j) != 1 {
		t.Errorf("download journal = %v", j)
	}
	if again := f.h.do(get(link, f.sess)); again.Code != http.StatusNotFound {
		t.Errorf("second follow: %d", again.Code)
	}
}

// The link is the person's: another account's session gets the same 404 as
// a dead link, and the link still opens for its owner.
func TestProfileLinkIsThePersons(t *testing.T) {
	f, _ := deviceFixture(t)
	unlock(t, f)
	got := decode[view.DevicesPage](t, f.h.do(postForm("/app/settings/devices?tab=apple&op=profile", f.sess, url.Values{})))
	var link string
	for _, n := range got.Notices {
		for _, a := range n.Actions {
			if a.ID == "device.profile.download" {
				link = a.Href
			}
		}
	}
	snap := f.h.st.Snapshot()
	adminID := snap.Users[0].ID
	f.h.invite(adminID, "mallory", "password456")
	mallory := f.h.login("mallory", "password456")
	if rec := f.h.do(get(link, mallory)); rec.Code != http.StatusNotFound {
		t.Errorf("someone else followed the link: %d", rec.Code)
	}
	if rec := f.h.do(get(link, f.sess)); rec.Code != http.StatusOK {
		t.Errorf("the owner after a stranger: %d", rec.Code)
	}
	if rec := f.h.do(get("/app/settings/devices/profile?t=AAAA", f.sess)); rec.Code != http.StatusNotFound {
		t.Errorf("a made-up link: %d", rec.Code)
	}
}

// A connection left out is in no profile and marked on every tab; with
// nothing left, the profile is not offered.
func TestLeftOutAccountsGoNowhere(t *testing.T) {
	f, otherID := deviceFixture(t)
	p := devicesPage(t, f, "apple")
	rec := f.h.do(postForm(deviceAction(t, p.Actions, "device.exclude").Href, f.sess, url.Values{}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("exclude: %d", rec.Code)
	}
	p = devicesPage(t, f, "thunderbird")
	excluded := 0
	for _, a := range p.Accounts {
		for _, r := range a.Rows {
			if !r.Included {
				excluded++
			}
		}
	}
	if excluded == 0 {
		t.Fatalf("nothing marked left out: %+v", p.Accounts)
	}
	unlock(t, f)
	got := decode[view.DevicesPage](t, f.h.do(postForm("/app/settings/devices?tab=apple&op=profile", f.sess, url.Values{})))
	var link string
	for _, n := range got.Notices {
		for _, a := range n.Actions {
			if a.ID == "device.profile.download" {
				link = a.Href
			}
		}
	}
	body := f.h.do(get(link, f.sess)).Body.String()
	if strings.Contains(body, "<string>alice</string>") || !strings.Contains(body, "<string>bob</string>") {
		t.Errorf("the left-out account is in the profile:\n%s", body)
	}
	f.h.do(postForm("/app/settings/devices?op=exclude&account="+otherID, f.sess, url.Values{}))
	if gen := deviceAction(t, devicesPage(t, f, "apple").Actions, "device.profile"); gen.Enabled {
		t.Errorf("a profile of nothing is offered: %+v", gen)
	}
}

// The tabs: addresses whole, with Copy; a file collection only under
// "anything else".
func TestDeviceTabs(t *testing.T) {
	f, _ := deviceFixture(t)
	p := devicesPage(t, f, "thunderbird")
	if p.Tab != view.TabThunderbird || len(p.Accounts) != 2 {
		t.Fatalf("thunderbird = %+v", p)
	}
	types := map[string]string{}
	for _, a := range p.Accounts {
		for _, r := range a.Rows {
			types[string(r.Label)] = string(r.Type)
			if !strings.HasPrefix(string(r.URL), "http") || r.Copy.Href != string(r.URL) {
				t.Errorf("row = %+v", r)
			}
		}
	}
	if types["Work"] != "CalDAV" || types["People"] != "CardDAV" {
		t.Errorf("types = %v", types)
	}
	for _, a := range p.Accounts {
		if a.QR != "" {
			t.Errorf("a code outside the Android tab: %.40q", a.QR)
		}
	}
}

// Android: a code for each ticked account, all on the page at once — DAVx5
// makes an account per code (owner's decision 24.09.2026, s-plan-2). Each
// opens DAVx5 with the address and the login, never the password, and has
// «Open in DAVx5» beside it. A left-out account has no code.
func TestDAVx5Code(t *testing.T) {
	f, otherID := deviceFixture(t)
	p := devicesPage(t, f, "android")
	var coded []view.DeviceAccount
	for _, a := range p.Accounts {
		if a.QR != "" {
			coded = append(coded, a)
		}
	}
	if len(coded) != 2 || coded[0].QR == coded[1].QR {
		t.Fatalf("codes: %d accounts of %d", len(coded), len(p.Accounts))
	}
	img, ok := strings.CutPrefix(coded[0].QR, "data:image/png;base64,")
	if !ok {
		t.Fatalf("QR = %.60q", coded[0].QR)
	}
	raw, err := base64.StdEncoding.DecodeString(img)
	if err != nil {
		t.Fatal(err)
	}
	pic, err := png.Decode(bytes.NewReader(raw))
	if err != nil || pic.Bounds().Dx() < 100 || pic.Bounds().Dx() != pic.Bounds().Dy() {
		t.Fatalf("code image: %v %v", pic, err)
	}
	open := deviceAction(t, coded[0].Actions, "device.davx5")
	u, err := url.Parse(open.Href)
	if err != nil || u.Scheme != "caldav" || u.User.Username() != "alice" || strings.Contains(open.Href, "s3cret") {
		t.Errorf("DAVx5 link = %q", open.Href) // the test server is plain HTTP: caldav://, not davx5://
	}
	if !strings.Contains(deviceAction(t, coded[1].Actions, "device.davx5").Href, "bob@") {
		t.Errorf("the other account's code: %+v", coded[1].Actions)
	}
	for _, a := range p.Actions {
		if a.ID == "device.code" {
			t.Errorf("a code one link away is gone: %+v", a)
		}
	}
	f.h.do(postForm("/app/settings/devices?op=exclude&account="+otherID, f.sess, url.Values{}))
	n := 0
	for _, a := range devicesPage(t, f, "android").Accounts {
		if a.QR != "" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("a left-out account keeps its code: %d codes", n)
	}
}

// The password of each account is on the page it is needed on (owner's
// decision 24.09.2026, s-plan-2, s-plan-4, s-plan-5): under its code on
// Android, in its row with the login on Thunderbird and «Anything else».
// Outside the unlock window it is not in the answer at all — Show opens the
// sheet, and Copy waits for it; inside, it is there, and every showing is
// journaled. Apple has no password of its own.
func TestDevicePasswordsOnThePage(t *testing.T) {
	f, _ := deviceFixture(t)
	for _, tab := range []string{"android", "thunderbird", "other"} {
		p := devicesPage(t, f, tab)
		a := p.Accounts[0]
		if a.Password.Shown || a.Password.Value != "" || !strings.HasSuffix(a.Password.Reveal.Href, "/app/settings/unlock") {
			t.Errorf("%s, locked: password = %+v", tab, a.Password)
		}
		if c := deviceAction(t, a.Actions, "device.password.copy"); c.Enabled {
			t.Errorf("%s, locked: copy is enabled", tab)
		}
		if tab != "android" && a.Login != "alice" {
			t.Errorf("%s: login = %q", tab, a.Login)
		}
	}
	for _, a := range devicesPage(t, f, "apple").Accounts {
		if a.Password.Value != "" || a.Password.Reveal.ID != "" {
			t.Errorf("apple has a password field: %+v", a.Password)
		}
	}

	unlock(t, f)
	before := auditCount(f.h, "connection.password.reveal")
	p := devicesPage(t, f, "thunderbird")
	var alice view.DeviceAccount
	for _, a := range p.Accounts {
		if a.Login == "alice" {
			alice = a
		}
	}
	if !alice.Password.Shown || alice.Password.Value != "s3cret&<pass>" || !deviceAction(t, alice.Actions, "device.password.copy").Enabled {
		t.Errorf("unlocked: %+v", alice)
	}
	if got := auditCount(f.h, "connection.password.reveal") - before; got != len(p.Accounts) {
		t.Errorf("showings journaled: %d, accounts shown: %d", got, len(p.Accounts))
	}
}
