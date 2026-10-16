package web_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// formsWithoutButtons walks a JSON view model and names every form that goes
// somewhere and offers nothing to press. The words on a button are the
// server's to give (C-3.1): a client that has to guess "Submit" is guessing
// the product.
func formsWithoutButtons(t *testing.T, rec *httptest.ResponseRecorder) []string {
	t.Helper()
	var v any
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var out []string
	var walk func(v any, path string)
	walk = func(v any, path string) {
		switch x := v.(type) {
		case map[string]any:
			if _, ok := x["sets"]; ok {
				acts, _ := x["actions"].([]any)
				// A form that posts; a filter (GET) goes by Enter (Ад6).
				if action, _ := x["action"].(string); action != "" && len(acts) == 0 && x["method"] == "POST" {
					out = append(out, path+" → "+action)
				}
			}
			for k, c := range x {
				walk(c, path+"."+k)
			}
		case []any:
			for _, c := range x {
				walk(c, path+"[]")
			}
		}
	}
	walk(v, "")
	return out
}

// Every form that posts has its button, with the words the frames give it.
// Sign in, Create administrator, Change password, Connect, Invite, the escrow
// forms, the backup job and the unlock sheet had none.
func TestEveryFormHasItsButton(t *testing.T) {
	h := newHarness(t)
	for _, p := range []string{"/setup"} {
		if bad := formsWithoutButtons(t, h.do(httptest.NewRequest(http.MethodGet, p, nil))); len(bad) > 0 {
			t.Errorf("%s: %v", p, bad)
		}
	}
	h.admin("owner", "password123")
	for _, p := range []string{"/login", "/forgot"} {
		if bad := formsWithoutButtons(t, h.do(httptest.NewRequest(http.MethodGet, p, nil))); len(bad) > 0 {
			t.Errorf("%s: %v", p, bad)
		}
	}
	sess := h.login("owner", "password123")
	for _, p := range []string{
		"/app/password", "/app/settings/connections", "/app/settings/unlock",
		"/app/settings/backup", "/admin/", "/admin/invites", "/admin/escrow",
	} {
		if bad := formsWithoutButtons(t, h.do(get(p, sess))); len(bad) > 0 {
			t.Errorf("%s: %v", p, bad)
		}
	}
}
