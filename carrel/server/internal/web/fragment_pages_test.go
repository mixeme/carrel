package web_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"carrel.dev/carrel/server/view"
)

// pageFragment is the envelope of a fragment whose body is a whole page.
type pageFragment[T any] struct {
	Target string `json:"target"`
	Body   T      `json:"body"`
}

// bodyKeys are the keys of a fragment's body: a page in the shell has top
// and rail beside its own fields.
func bodyKeys(t *testing.T, raw []byte) map[string]json.RawMessage {
	t.Helper()
	var env struct {
		Body map[string]json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return env.Body
}

// The lead's decision П2 (stage 4 of the client's plan): the decision on a
// group of duplicates, Test only beside the connection form and the test
// letter answer with their whole page, not with a fragment that is no zone
// of C-7.2 — DuplicateGroup, DiagnosticTrace and MailTestResult had no
// target to land in, and the decision went out as «main». Asked for a
// fragment, each answers its page, and the client cuts the zone asked for;
// Test only without scripts is the page too (it was 404).
func TestAnswersOutsideTheZonesArePages(t *testing.T) {
	t.Run("connection test", func(t *testing.T) {
		srv := checkServer(t)
		h, sess := davHarness(t)
		form := url.Values{"server": {srv.URL}, "login": {"alice"}, "password": {"davpass"}, "label": {"home"}}
		rec := h.do(postForm("/app/settings/connections/test", sess, form))
		if rec.Code != http.StatusOK {
			t.Fatalf("Test only without scripts: %d %s", rec.Code, rec.Body)
		}
		page := decode[view.ConnectionsPage](t, rec)
		if page.Trace == nil || !page.Trace.OK || page.Add == nil {
			t.Fatalf("page = trace %+v, dialog %v", page.Trace, page.Add != nil)
		}
		for _, set := range page.Add.Sets {
			for _, row := range set.Rows {
				if row.Name == "server" && string(row.Value) != srv.URL || row.Name == "label" && row.Value != "home" || row.Name == "password" && row.Value != "" {
					t.Errorf("dialog row %s = %q: the typed values come back, the password does not", row.Name, row.Value)
				}
			}
		}
		raw := h.do(fragment(postForm("/app/settings/connections/test", sess, form), "main")).Body.Bytes()
		keys := bodyKeys(t, raw)
		if _, ok := keys["accounts"]; !ok {
			t.Errorf("Test only as a fragment answers %s, want the connections page", raw)
		}
		key, _ := sess.DataKey()
		if blob, _ := h.users.Catalog(sess.AccountID(), key); len(blob.Connections) != 0 {
			t.Error("Test only saved a connection")
		}
	})
	t.Run("mail test", func(t *testing.T) {
		h := newHarness(t)
		h.admin("alice", "password123")
		alice := h.login("alice", "password123")
		// The test letter sends the form as typed (Ад37).
		raw := h.do(fragment(postForm("/admin/settings/mail-test", alice, settingsForm(t, h, alice, map[string]string{"to": "alice@example.org"})), "main")).Body.Bytes()
		keys := bodyKeys(t, raw)
		if _, ok := keys["form"]; !ok {
			t.Errorf("the test letter as a fragment answers %s, want the settings page", raw)
		}
		if _, ok := keys["test"]; !ok {
			t.Errorf("the settings page carries no Test: %s", raw)
		}
	})
	t.Run("duplicates", func(t *testing.T) {
		f := newDupFixture(t)
		f.load(t)
		g := onlyGroup(t, f.page(t))
		rec := f.h.do(fragment(postForm(actionOf(t, g, "duplicates.link").Href, f.sess, url.Values{}), "main"))
		got := decode[pageFragment[view.DuplicatesPage]](t, rec)
		if got.Target != "main" || len(got.Body.Groups) != 1 || got.Body.Groups[0].Decision != view.DecisionLinked || len(got.Body.Rail.Nav) == 0 {
			t.Errorf("a decision as a fragment answers %s, want the duplicates page with the group linked", rec.Body)
		}
	})
}
