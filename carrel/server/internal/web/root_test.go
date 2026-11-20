package web_test

import (
	"net/http"
	"testing"
)

// The bare address of an instance (https://carrel.mixfed.ru/) leads into
// the application, as /app/ does: a person signed in to their contacts,
// anyone else to the sign-in — and on an empty instance to the setup. It
// answered 404 in plain text: the first page of the first deploy (v0.4.0).
func TestTheBareAddressLeadsIn(t *testing.T) {
	h := newHarness(t)
	rec := h.do(get("/", nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/app/" {
		t.Fatalf("GET / = %d %q, want 303 to /app/", rec.Code, rec.Header().Get("Location"))
	}
	if rec := h.do(get("/nothing-here", nil)); rec.Code != http.StatusNotFound {
		t.Errorf("an address that is nothing = %d, want 404", rec.Code)
	}
}
