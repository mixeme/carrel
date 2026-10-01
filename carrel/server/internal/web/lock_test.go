package web_test

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"carrel.dev/carrel/server/view"
)

// Lock now (s-profile-4, s-plan-1…5, S-10.7): beside «credentials unlocked
// · 4:12» stands the button that ends the unlock window of this session at
// once — the secrets are asked for again, the sign-in stays —, and the
// messages say so in the words of the frames, Lock now in bold. Outside the
// window there is nothing to lock: no button, no message.
func TestLockNowEndsTheUnlockNotTheSignIn(t *testing.T) {
	f, _ := deviceFixture(t)
	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	if page.Top.Lock != nil || len(page.Note) != 0 {
		t.Fatalf("locked: Lock %+v, note %+v", page.Top.Lock, page.Note)
	}

	unlock(t, f)
	page = decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	lock := page.Top.Lock
	if page.Top.Unlocked == nil || lock == nil || lock.ID != "unlock.lock" || lock.Label != "Lock now" || lock.Method != "POST" || lock.Href != "/app/settings/lock" || !lock.Enabled {
		t.Fatalf("unlocked: until %v, Lock %+v", page.Top.Unlocked, lock)
	}
	if got := plainRuns(page.Note); got != "While credentials are unlocked, every account opens without a question — here, on the New device screen and when a profile is built. Lock now ends it at once; so does signing out." ||
		!hasRun(page.Note, "Lock now", view.RunStrong) {
		t.Errorf("the connections' message = %+v", page.Note)
	}
	other := decode[view.DevicesPage](t, f.h.do(get("/app/settings/devices?tab=other", f.sess)))
	if got := plainRuns(other.UnlockNote); got != "Credentials are unlocked for five minutes — here and everywhere else they are handed out. Everything shown closes when the time is up or when you press Lock now; every reveal is written to the audit log." ||
		!hasRun(other.UnlockNote, "Lock now", view.RunStrong) {
		t.Errorf("the devices' message = %+v", other.UnlockNote)
	}
	if other.Top.Lock == nil {
		t.Error("the devices' head has no Lock now")
	}

	req := postForm("/app/settings/lock", f.sess, url.Values{})
	req.Header.Set("Referer", "http://example.com/app/settings/devices?tab=other")
	rec := f.h.do(req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/app/settings/devices?tab=other" {
		t.Fatalf("lock: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if until := f.sess.UnlockUntil(time.Now()); until != nil {
		t.Errorf("the window is open after Lock now, until %v", until)
	}
	after := decode[view.DevicesPage](t, f.h.do(get("/app/settings/devices?tab=other", f.sess)))
	if after.Top.Unlocked != nil || after.Top.Lock != nil || len(after.UnlockNote) != 0 {
		t.Errorf("after Lock now: until %v, Lock %+v, note %+v", after.Top.Unlocked, after.Top.Lock, after.UnlockNote)
	}
	for _, a := range after.Accounts {
		if a.Password.Shown || a.Password.Value != "" {
			t.Errorf("a password is still shown after Lock now: %+v", a.Password)
		}
	}

	// A Referer from elsewhere, or none, is not followed.
	for _, ref := range []string{"https://evil.example/app/x", "", "http://example.com//evil.example/"} {
		req := postForm("/app/settings/lock", f.sess, url.Values{})
		if ref != "" {
			req.Header.Set("Referer", ref)
		}
		if rec := f.h.do(req); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/app/settings/connections" {
			t.Errorf("Referer %q: %d %q", ref, rec.Code, rec.Header().Get("Location"))
		}
	}
}

// hasRun says whether runs hold text as one piece marked mark.
func hasRun(runs view.Runs, text string, mark view.RunMark) bool {
	for _, r := range runs {
		if string(r.Text) == text && r.Mark == mark {
			return true
		}
	}
	return false
}

// plainRuns is the sentence of runs without its marks.
func plainRuns(runs view.Runs) string {
	out := ""
	for _, r := range runs {
		out += string(r.Text)
	}
	return out
}
