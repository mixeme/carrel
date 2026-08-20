// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/store"
)

func TestEmptySectionRailOffersNewCollection(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "", testPassword)
	a.putTestBook(t, testBookAccount("acc-test", "http://127.0.0.1/"))

	for _, tc := range []struct {
		path, label string
	}{
		{"/app/calendar", "New calendar"},
		{"/app/tasks", "New list"},
		{"/app/notes", "New notebook"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			rec := a.get(tc.path)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if !strings.Contains(body, tc.label) {
				t.Fatalf("empty rail hid %q:\n%s", tc.label, body)
			}
			if !strings.Contains(body, "/app/collections/new") {
				t.Fatalf("empty rail has no create URL:\n%s", body)
			}
		})
	}
}

func TestEmptyContactsRailOffersNewAddressBook(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "", testPassword)
	a.putTestBook(t, testCalAccount("acc-test", "http://127.0.0.1/"))

	rec := a.get("/app/contacts")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "New address book") {
		t.Fatalf("empty contacts rail hid New address book:\n%s", body)
	}
	if !strings.Contains(body, "/app/collections/new") {
		t.Fatalf("empty contacts rail has no create URL:\n%s", body)
	}
}

func TestNewCollectionFormOffersAddressBook(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "", testPassword)
	a.putTestBook(t, testBookAccount("acc-test", "http://127.0.0.1/"))

	rec := a.get("/app/collections/new?account=acc-test&return=/app/settings/connections")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Address book") {
		t.Fatalf("new-collection form has no address-book kind:\n%s", body)
	}
	if !strings.Contains(body, "kind=addressbook") {
		t.Fatalf("new-collection form has no address-book URL:\n%s", body)
	}
}

func TestConnectionsCollectionRowHasDotsMenu(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "", testPassword)
	a.putTestBook(t, testBookAccount("acc-test", "http://127.0.0.1/"))

	rec := a.get("/app/settings/connections")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `data-dots-menu`) {
		t.Fatalf("collection row has no ⋯ menu:\n%s", body)
	}
	if !strings.Contains(body, "Collection actions") {
		t.Fatalf("⋯ toggle missing:\n%s", body)
	}
}

func TestDeleteFormListsReferenceRows(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "", testPassword)
	acc := testBookAccount("acc-test", "http://127.0.0.1/")
	a.putTestBook(t, acc)
	colEnc := EncodeCollectionPath(acc.Collections[0].Path)

	rec := a.get("/app/collections/delete?account=acc-test&col=" + url.QueryEscape(colEnc))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "What is pointing at it now") {
		t.Fatalf("delete form missing reference list:\n%s", body)
	}
	for _, row := range []string{"No published links", "No backup jobs", "No davloom devices"} {
		if !strings.Contains(body, row) {
			t.Fatalf("delete form missing %q:\n%s", row, body)
		}
	}
}

func TestCollectionDeleteRequiresTypedName(t *testing.T) {
	var deletes atomic.Int32
	davSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deletes.Add(1)
		}
		http.Error(w, "unused", http.StatusNotImplemented)
	}))
	defer davSrv.Close()

	a := newApp(t, nil)
	a.Guard = dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	a.setupAdmin("root", "", testPassword)
	acc := testBookAccount("acc-test", davSrv.URL+"/")
	a.putTestBook(t, acc)

	rec := a.post("/app/collections/delete", url.Values{
		"account_id":         {acc.ID},
		"collection_path":    {acc.Collections[0].Path},
		"confirm_collection": {"not the name"},
		"return":             {"/app/settings/connections"},
		"mode":               {"delete"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "type the collection name") {
		t.Fatalf("mismatch did not ask to type the name:\n%s", rec.Body.String())
	}
	if deletes.Load() != 0 {
		t.Fatalf("DELETE reached the server %d times", deletes.Load())
	}
	stored, err := a.Store.GetDAVAccount(a.session().UserID, acc.ID, a.session().DEK())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := discovery.FindCollection(stored.Collections, acc.Collections[0].Path); !ok {
		t.Fatal("collection was removed after a mismatched name")
	}
}

func TestUserCannotDeleteAnotherUsersCollection(t *testing.T) {
	var deletes atomic.Int32
	davSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deletes.Add(1)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer davSrv.Close()

	a := newApp(t, nil)
	a.Guard = dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	a.setupAdmin("root", "", testPassword)
	admin := a.session()
	acc := testBookAccount("acc-test", davSrv.URL+"/")
	a.putTestBook(t, acc)

	if _, err := a.Store.CreateUserWithPassword(
		store.Actor{ID: admin.UserID, Login: admin.Login}, "ada", "", store.RoleUser, testPassword,
	); err != nil {
		t.Fatalf("CreateUserWithPassword: %v", err)
	}
	a.signInReady("ada", testPassword)

	colEnc := EncodeCollectionPath(acc.Collections[0].Path)
	get := a.get("/app/collections/delete?account=" + acc.ID + "&col=" + url.QueryEscape(colEnc))
	if get.Code != http.StatusBadRequest {
		t.Fatalf("GET status = %d, want 400, body = %s", get.Code, get.Body.String())
	}
	if !strings.Contains(get.Body.String(), "account not found") {
		t.Fatalf("GET body = %s", get.Body.String())
	}

	post := a.post("/app/collections/delete", url.Values{
		"account_id":         {acc.ID},
		"collection_path":    {acc.Collections[0].Path},
		"confirm_collection": {"Personal"},
		"return":             {"/app/settings/connections"},
		"mode":               {"delete"},
	})
	if post.Code != http.StatusBadRequest {
		t.Fatalf("POST status = %d, want 400, body = %s", post.Code, post.Body.String())
	}
	if deletes.Load() != 0 {
		t.Fatalf("DELETE reached the server %d times", deletes.Load())
	}
}

func testCalAccount(id, baseURL string) account.Account {
	return account.Account{
		ID:        id,
		Label:     "Test",
		BaseURL:   baseURL,
		Username:  "mix",
		Password:  "secret",
		Principal: "/principals/mix/",
		Enabled:   true,
		Collections: []discovery.Collection{{
			Path:        "/calendars/user/default/",
			DisplayName: "Personal",
			Kind:        discovery.KindCalendar,
		}},
	}
}

func testBookAccount(id, baseURL string) account.Account {
	return account.Account{
		ID:        id,
		Label:     "Test",
		BaseURL:   baseURL,
		Username:  "mix",
		Password:  "secret",
		Principal: "/principals/mix/",
		Enabled:   true,
		Collections: []discovery.Collection{{
			Path:        "/addressbooks/user/default/",
			DisplayName: "Personal",
			Kind:        discovery.KindAddressBook,
		}},
	}
}

func (a *app) putTestBook(t *testing.T, acc account.Account) {
	t.Helper()
	sess := a.session()
	actor := store.Actor{ID: sess.UserID, Login: sess.Login}
	if err := a.Store.PutDAVAccount(actor, sess.UserID, sess.DEK(), acc); err != nil {
		t.Fatal(err)
	}
}
