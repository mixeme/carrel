package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// s-profile-3 (Нс18–Нс20): Show password of a connection opens the sheet
// that says whose password it asks for — «Show password», the connection —,
// one line on what the window opens and for how long, and Show and Cancel.
func TestThePasswordSheetSaysWhose(t *testing.T) {
	f := calendarFixture(t, nil)
	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	pw := page.Accounts[0].Password
	if pw == nil || !strings.Contains(pw.Reveal.Href, "for="+f.id) {
		t.Fatalf("Show password = %+v", pw)
	}
	sheet := decode[view.UnlockSheet](t, f.h.do(get(pw.Reveal.Href, f.sess)))
	if sheet.Purpose != "Show password" || sheet.Subject != view.Text(page.Accounts[0].Title) {
		t.Errorf("head = %q · %q, want «Show password» · %q", sheet.Purpose, sheet.Subject, page.Accounts[0].Title)
	}
	if sheet.Warning != "Unlocks stored credentials for five minutes." {
		t.Errorf("line = %q", sheet.Warning)
	}
	if a := sheet.Form.Actions; len(a) != 2 || a[0].ID != "unlock.submit" || a[0].Label != "Show" || a[1].ID != "unlock.cancel" || a[1].Href == "" {
		t.Errorf("actions = %+v", a)
	}
}

// Typecase's note 86: one sheet for every handing out — Generate profile
// with nothing on its right, which issues the profile's link at once; the
// title what for, the main button the act.
func TestThePasswordSheetDoesWhatItIsFor(t *testing.T) {
	f, _ := deviceFixture(t)
	gen := deviceAction(t, devicesPage(t, f, "apple").Actions, "device.profile")
	sheet := decode[view.UnlockSheet](t, f.h.do(get(gen.Href, f.sess)))
	if sheet.Purpose != "Generate profile" || sheet.Subject != "" || sheet.Form.Actions[0].Label != "Generate" ||
		sheet.Form.Actions[1].Href != "/app/settings/devices?tab=apple" {
		t.Fatalf("the sheet = %+v", sheet)
	}
	rec := f.h.do(postForm("/app/settings/unlock", f.sess, url.Values{"for": {"profile"}, "back": {"/app/settings/devices?tab=apple"}, "password": {"password123"}}))
	if rec.Code != 303 || rec.Header().Get("Location") != "/app/settings/devices?tab=apple" {
		t.Fatalf("Generate: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if len(journaled(f, "device.profile.link")) != 1 {
		t.Error("Generate issued no link")
	}
	if p := devicesPage(t, f, "apple"); p.Link == nil {
		t.Error("the page shows no link after Generate")
	}
}

// 193 p. 3, Typecase's notes 86 and 91: outside the window Turn on of the
// trigger's sheet is the password's sheet over the list of jobs —
// «External trigger», the job on its right, Turn on —, and the password's
// sheet turns it on: the trigger's sheet with the password and the line
// whole, once.
func TestThePasswordSheetTurnsTheTriggerOn(t *testing.T) {
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "pw"))
	c := triggerSheetOf(t, f, job.ID, "").Form.Actions[0]
	rec := f.h.do(postForm(c.Href, f.sess, url.Values{}))
	if c.Method != http.MethodPost || rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "for=trigger") {
		t.Fatalf("Turn on outside the window = %+v: %d %s", c, rec.Code, rec.Header().Get("Location"))
	}
	sheet := decode[view.UnlockSheet](t, f.h.do(get(rec.Header().Get("Location"), f.sess)))
	if sheet.Purpose != "External trigger" || sheet.Subject != view.Text(job.Name) || sheet.Form.Actions[0].Label != "Turn on" || sheet.Under == nil || sheet.Under.Backup == nil {
		t.Fatalf("the sheet = %+v", sheet)
	}
	back := "/app/settings/backup/trigger?job=" + job.ID
	rec = f.h.do(postForm("/app/settings/unlock", f.sess, url.Values{"for": {"trigger"}, "job": {job.ID}, "back": {back}, "password": {"password123"}}))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != back {
		t.Fatalf("Turn on: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if issued := triggerSheetOf(t, f, job.ID, ""); issued.Password == nil || issued.Password.Value == "" || issued.Line == nil || !issued.Line.Shown {
		t.Fatalf("the trigger after the sheet = %+v", issued)
	}
	if again := triggerSheetOf(t, f, job.ID, ""); again.Password == nil || again.Password.Value != "" {
		t.Error("the password is shown twice")
	}
}
