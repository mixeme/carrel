// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/store"
)

func TestContactsListAndPhotoPlaceholder(t *testing.T) {
	davSrv := startCardDAVBook(t)
	defer davSrv.Close()

	a := newApp(t, nil)
	a.Guard = dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	a.setupAdmin("root", "", testPassword)
	accID, colEnc := a.connectAddressBook(t, davSrv.URL)

	rec := a.get("/app/contacts/" + accID + "/" + colEnc)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Ada Lovelace") {
		t.Fatalf("list missing contact: %s", body)
	}

	photo := a.get("/c/" + accID + "/" + colEnc + "/ada/photo?size=thumb")
	if photo.Code != http.StatusOK {
		t.Fatalf("photo status = %d", photo.Code)
	}
	ct := photo.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "image/svg+xml") {
		t.Fatalf("Content-Type = %q, want svg placeholder", ct)
	}
}

func TestContactSaveKeepsUnknownProperties(t *testing.T) {
	davSrv := startCardDAVBook(t)
	defer davSrv.Close()

	a := newApp(t, nil)
	a.Guard = dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	a.setupAdmin("root", "", testPassword)
	accID, colEnc := a.connectAddressBook(t, davSrv.URL)

	rec := a.post("/app/contacts/"+accID+"/"+colEnc+"/ada", url.Values{
		fieldAction: {"save"},
		"etag":      {`"v1"`},
		"fn":        {"Ada Lovelace"},
		"given":     {"Ada"},
		"family":    {"Lovelace"},
		"phone":     {"+1-555"},
		"email":     {"ada@example.org"},
	})
	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusOK {
		t.Fatalf("save status = %d, body = %s", rec.Code, rec.Body.String())
	}

	card := a.get("/app/contacts/" + accID + "/" + colEnc + "/ada")
	if card.Code != http.StatusOK {
		t.Fatalf("card status = %d", card.Code)
	}
	if !strings.Contains(card.Body.String(), "X-CUSTOM") {
		t.Fatalf("X-CUSTOM missing from card:\n%s", card.Body.String())
	}
}

func TestContactConflictShowsChoice(t *testing.T) {
	davSrv := startCardDAVBook(t)
	defer davSrv.Close()
	davSrv.failNextPut = true

	a := newApp(t, nil)
	a.Guard = dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	a.setupAdmin("root", "", testPassword)
	accID, colEnc := a.connectAddressBook(t, davSrv.URL)

	rec := a.post("/app/contacts/"+accID+"/"+colEnc+"/ada", url.Values{
		fieldAction: {"save"},
		"etag":      {`"stale"`},
		"fn":        {"Ada Changed"},
		"given":     {"Ada"},
		"family":    {"Changed"},
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Conflict") || !strings.Contains(body, "Keep server version") {
		t.Fatalf("conflict UI missing:\n%s", body)
	}
	if !strings.Contains(body, "Apply my edit") {
		t.Fatal("apply-mine choice missing")
	}
}

func (a *app) connectAddressBook(t *testing.T, baseURL string) (accountID, colEnc string) {
	t.Helper()
	sess := a.session()
	actor := store.Actor{ID: sess.UserID, Login: sess.Login}
	acc := account.Account{
		ID:       "acc-test",
		Label:    "Test",
		BaseURL:  baseURL + "/",
		Username: "mix",
		Password: "secret",
		Enabled:  true,
		Collections: []discovery.Collection{{
			Path:        "/addressbooks/user/default/",
			DisplayName: "Personal",
			Kind:        discovery.KindAddressBook,
		}},
	}
	if err := a.Store.PutDAVAccount(actor, sess.UserID, sess.DEK(), acc); err != nil {
		t.Fatal(err)
	}
	return acc.ID, EncodeCollectionPath(acc.Collections[0].Path)
}

type cardDAVBook struct {
	*httptest.Server
	mu          sync.Mutex
	cards       map[string]string
	etags       map[string]string
	failNextPut bool
}

func startCardDAVBook(t *testing.T) *cardDAVBook {
	t.Helper()
	book := &cardDAVBook{
		cards: map[string]string{
			"/addressbooks/user/default/ada.vcf": "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:ada\r\nFN:Ada Lovelace\r\nN:Lovelace;Ada;;;\r\nTEL:+1-555\r\nEMAIL:ada@example.org\r\nX-CUSTOM:keep-me\r\nEND:VCARD\r\n",
		},
		etags: map[string]string{
			"/addressbooks/user/default/ada.vcf": `"v1"`,
		},
	}
	book.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "" {
			path = "/"
		}
		book.mu.Lock()
		defer book.mu.Unlock()
		switch {
		case r.Method == "PROPFIND" && (path == "/addressbooks/user/default/" || path == "/addressbooks/user/default"):
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusMultiStatus)
			io.WriteString(w, `<?xml version="1.0"?>
<d:multistatus xmlns:d="DAV:" xmlns:cs="http://calendarserver.org/ns/">
  <d:response>
    <d:href>/addressbooks/user/default/</d:href>
    <d:propstat><d:prop><cs:getctag>ctag-1</cs:getctag></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat>
  </d:response>
  <d:response>
    <d:href>/addressbooks/user/default/ada.vcf</d:href>
    <d:propstat><d:prop><d:getetag>"v1"</d:getetag></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat>
  </d:response>
</d:multistatus>`)
		case r.Method == "REPORT":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusMultiStatus)
			body := book.cards["/addressbooks/user/default/ada.vcf"]
			etag := book.etags["/addressbooks/user/default/ada.vcf"]
			io.WriteString(w, `<?xml version="1.0"?>
<d:multistatus xmlns:d="DAV:" xmlns:card="urn:ietf:params:xml:ns:carddav">
  <d:response>
    <d:href>/addressbooks/user/default/ada.vcf</d:href>
    <d:propstat><d:prop><d:getetag>`+etag+`</d:getetag><card:address-data>`+xmlEscapeText(body)+`</card:address-data></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat>
  </d:response>
</d:multistatus>`)
		case r.Method == http.MethodGet && strings.HasSuffix(path, ".vcf"):
			body, ok := book.cards[path]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("ETag", book.etags[path])
			w.Header().Set("Content-Type", "text/vcard")
			io.WriteString(w, body)
		case r.Method == http.MethodPut:
			if book.failNextPut {
				book.failNextPut = false
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}
			raw, _ := io.ReadAll(r.Body)
			book.cards[path] = string(raw)
			book.etags[path] = `"v2"`
			w.Header().Set("ETag", `"v2"`)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete:
			delete(book.cards, path)
			delete(book.etags, path)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	return book
}

func xmlEscapeText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
