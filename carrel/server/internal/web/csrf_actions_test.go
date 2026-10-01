package web_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// asForm sends a POST action the way a page without scripts does: a form of
// its own to the action's address, its token as _csrf, no header.
func asForm(h *harness, sess *session.Session, a view.Action) *httptest.ResponseRecorder {
	form := url.Values{"_csrf": {a.CSRF}}
	req := httptest.NewRequest(http.MethodPost, a.Href, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: sess.ID()})
	return h.do(req)
}

// C-11, the client's stop C15: a POST action outside a form — Delete and
// Send of a card, Test of a connection, a sheet's own button, Sign out —
// goes as a form of its own with the token it carries, and the server takes
// it. Without the token the same form is refused.
func TestPostActionsGoWithoutScripts(t *testing.T) {
	app := newMockupApp(t)
	card := decode[view.ContactPage](t, app.h.do(get(app.listPath(t, "contacts", "kontakty")+"/4f2b-9c31-a0e7", app.sess)))
	conns := decode[view.ConnectionsPage](t, app.h.do(get("/app/settings/connections", app.sess)))
	var test view.Action
	for _, a := range conns.Accounts[0].Actions {
		if a.ID == "connection.test" {
			test = a
		}
	}
	del := actionByID(card.Actions, "record.delete")
	if del == nil || del.CSRF == "" || test.CSRF == "" {
		t.Fatalf("delete %+v, test %+v: a POST without the session's token", del, test)
	}
	bare := *del
	bare.CSRF = ""
	if rec := asForm(app.h, app.sess, bare); rec.Code != http.StatusForbidden {
		t.Fatalf("without the token: %d, want it refused", rec.Code)
	}
	for _, a := range []view.Action{test, *del} {
		if rec := asForm(app.h, app.sess, a); rec.Code == http.StatusForbidden || rec.Code == http.StatusBadRequest {
			t.Errorf("%s as a form of its own: %d %s", a.ID, rec.Code, rec.Body)
		}
	}
	var logout *view.Action
	for _, m := range card.Top.User {
		if m.ID == "user.logout" {
			logout = &m.Action
		}
	}
	if logout == nil || logout.CSRF == "" {
		t.Fatalf("sign out = %+v", logout)
	}
	if rec := asForm(app.h, app.sess, *logout); rec.Code != http.StatusSeeOther {
		t.Errorf("sign out as a form of its own: %d", rec.Code)
	}
	// A GET carries none.
	if more := card.More; more.CSRF != "" {
		t.Errorf("%s is a GET with a token", more.ID)
	}
}
