package web_test

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"carrel.dev/carrel/server/view"
)

// U-8: every screen works without scripts. The password sheet opened by a
// plain link is a page of its own (C-7.2), and its form is a plain POST:
// the right password opens the window and goes back to the page the sheet
// was opened over — named by the sheet's hidden back, taken from where the
// link was followed —; a wrong one is the sheet again, as a page, with the
// error at its field. It was a 404 (a fragment only).
func TestUnlockWithoutScripts(t *testing.T) {
	f, _ := deviceFixture(t)
	req := get("/app/settings/unlock", f.sess)
	req.Header.Set("Referer", "http://example.com/app/settings/devices?tab=other")
	sheet := decode[view.UnlockSheet](t, f.h.do(req))
	back := formRow(sheet.Form, "back")
	if back == nil || back.Kind != view.InputHidden || back.Value != "/app/settings/devices?tab=other" {
		t.Fatalf("the sheet as a page does not say where it was opened over: %+v", back)
	}

	wrong := f.h.do(postForm("/app/settings/unlock", f.sess, url.Values{"password": {"wrong-one"}, "back": {string(back.Value)}}))
	if wrong.Code != http.StatusOK {
		t.Fatalf("a wrong password without scripts: %d %s", wrong.Code, wrong.Body.String())
	}
	again := decode[view.UnlockSheet](t, wrong)
	if pw := formRow(again.Form, "password"); pw == nil || pw.Error != "Password is wrong." || formRow(again.Form, "back").Value != back.Value {
		t.Errorf("the sheet again = %+v", again.Form)
	}
	if f.sess.UnlockUntil(time.Now()) != nil {
		t.Fatal("a wrong password opened the window")
	}

	right := f.h.do(postForm("/app/settings/unlock", f.sess, url.Values{"password": {"password123"}, "back": {string(back.Value)}}))
	if right.Code != http.StatusSeeOther || right.Header().Get("Location") != "/app/settings/devices?tab=other" {
		t.Fatalf("the right password without scripts: %d %q", right.Code, right.Header().Get("Location"))
	}
	if f.sess.UnlockUntil(time.Now()) == nil {
		t.Error("the window is not open")
	}

	// A back that is not a page of this instance is not followed.
	// (Three: the password's bucket takes five a minute, these included.)
	for _, b := range []string{"https://evil.example/app/x", "//evil.example/app/", "/login"} {
		rec := f.h.do(postForm("/app/settings/unlock", f.sess, url.Values{"password": {"password123"}, "back": {b}}))
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/app/settings/connections" {
			t.Errorf("back %q: %d %q", b, rec.Code, rec.Header().Get("Location"))
		}
	}
}
