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

// 193 p. 3 (Typecase's note 86): outside the window the job's own clock is
// the password's sheet over the job's page — «External trigger», the job on
// its right, Turn on —, and the sheet turns it on: the line and its secret
// on the job's page, once.
func TestThePasswordSheetTurnsTheTriggerOn(t *testing.T) {
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "pw"))
	c := openJob(t, f).Clock
	if c.Method != http.MethodGet || !strings.Contains(c.Href, "for=trigger") {
		t.Fatalf("clock outside the window = %+v", c)
	}
	sheet := decode[view.UnlockSheet](t, f.h.do(get(c.Href, f.sess)))
	if sheet.Purpose != "External trigger" || sheet.Subject != view.Text(job.Name) || sheet.Form.Actions[0].Label != "Turn on" || sheet.Under == nil || sheet.Under.Backup == nil {
		t.Fatalf("the sheet = %+v", sheet)
	}
	back := "/app/settings/backup?job=" + job.ID
	rec := f.h.do(postForm("/app/settings/unlock", f.sess, url.Values{"for": {"trigger"}, "job": {job.ID}, "back": {back}, "password": {"password123"}}))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != back {
		t.Fatalf("Turn on: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	page := decode[view.BackupPage](t, f.h.do(get(back, f.sess)))
	if page.Job == nil || page.Job.Trigger == nil || page.Job.Trigger.Command == "" || !page.Job.Trigger.Password.Shown {
		t.Fatalf("the trigger after the sheet = %+v", page.Job)
	}
	if again := decode[view.BackupPage](t, f.h.do(get(back, f.sess))); again.Job.Trigger.Command != "" {
		t.Error("the secret is shown twice")
	}
}
