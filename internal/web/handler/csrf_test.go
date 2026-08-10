// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/session"
)

func postForm(target string, values url.Values) *http.Request {
	r := httptest.NewRequest(http.MethodPost, target, strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestCSRFSafeMethodsPass(t *testing.T) {
	s := newServer(t)
	chain := Chain(ok, s.LoadSession, s.CSRF)

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		rec := httptest.NewRecorder()
		chain.ServeHTTP(rec, httptest.NewRequest(method, "/login", nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s status = %d, want 200", method, rec.Code)
		}
	}
}

// The login and setup forms are posted by people who have no session yet, so
// they fall back to a double-submit cookie.
func TestCSRFAnonymousDoubleSubmit(t *testing.T) {
	s := newServer(t)

	var issued string
	chain := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issued = CSRFToken(r)
		w.WriteHeader(http.StatusOK)
	}), s.LoadSession, s.CSRF)

	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
	if issued == "" {
		t.Fatal("no CSRF token was made available to the form")
	}
	cookie := findCookie(t, rec.Result().Cookies(), CSRFCookie)
	if cookie.Value != issued {
		t.Fatal("the cookie and the token handed to the template differ")
	}
	if !cookie.HttpOnly {
		t.Error("the CSRF cookie is readable from script although the token is rendered server-side")
	}

	// Posting it back is accepted.
	rec = httptest.NewRecorder()
	req := postForm("/login", url.Values{CSRFField: {issued}})
	req.AddCookie(cookie)
	Chain(ok, s.LoadSession, s.CSRF).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("valid anonymous post status = %d, want 200", rec.Code)
	}

	// Without the cookie there is nothing to match against.
	rec = httptest.NewRecorder()
	Chain(ok, s.LoadSession, s.CSRF).ServeHTTP(rec, postForm("/login", url.Values{CSRFField: {issued}}))
	if rec.Code != http.StatusForbidden {
		t.Errorf("post without the cookie status = %d, want 403", rec.Code)
	}
}

func TestCSRFRejectsMissingAndWrongTokens(t *testing.T) {
	s := newServer(t)
	sess := login(t, s, session.User{ID: "u1", Login: "ada"})
	other := login(t, s, session.User{ID: "u2", Login: "bob"})
	chain := Chain(ok, s.LoadSession, s.CSRF)

	withSession := func(r *http.Request) *http.Request {
		r.AddCookie(&http.Cookie{Name: SessionCookie, Value: sess.ID})
		return r
	}

	cases := map[string]*http.Request{
		"no token":              withSession(postForm("/app/password", url.Values{})),
		"empty token":           withSession(postForm("/app/password", url.Values{CSRFField: {""}})),
		"another session token": withSession(postForm("/app/password", url.Values{CSRFField: {other.CSRF}})),
		"the session id":        withSession(postForm("/app/password", url.Values{CSRFField: {sess.ID}})),
	}
	for name, req := range cases {
		rec := httptest.NewRecorder()
		chain.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403", name, rec.Code)
		}
	}
}

func TestCSRFAcceptsSessionToken(t *testing.T) {
	s := newServer(t)
	sess := login(t, s, session.User{ID: "u1", Login: "ada"})
	chain := Chain(ok, s.LoadSession, s.CSRF)

	// A plain form posts the hidden field.
	rec := httptest.NewRecorder()
	req := postForm("/app/password", url.Values{CSRFField: {sess.CSRF}})
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: sess.ID})
	chain.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("form post status = %d, want 200", rec.Code)
	}

	// htmx sends the header instead, and fragments are checked the same way
	// as full page posts (§24.5).
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/app/password", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set(CSRFHeader, sess.CSRF)
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: sess.ID})
	chain.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("htmx post status = %d, want 200", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/app/password", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set(CSRFHeader, "wrong")
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: sess.ID})
	chain.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("htmx post with a wrong header status = %d, want 403", rec.Code)
	}
}

// A session's token, not the anonymous cookie, is what counts once logged in:
// an attacker who can plant a cookie must not thereby choose the token.
func TestCSRFSessionTokenBeatsCookie(t *testing.T) {
	s := newServer(t)
	sess := login(t, s, session.User{ID: "u1", Login: "ada"})

	rec := httptest.NewRecorder()
	req := postForm("/app/password", url.Values{CSRFField: {"planted"}})
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: sess.ID})
	req.AddCookie(&http.Cookie{Name: CSRFCookie, Value: "planted"})
	Chain(ok, s.LoadSession, s.CSRF).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403: a planted cookie must not stand in for the session token", rec.Code)
	}
}

func TestCSRFTokenReachesHandler(t *testing.T) {
	s := newServer(t)
	sess := login(t, s, session.User{ID: "u1", Login: "ada"})

	var got string
	chain := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = CSRFToken(r)
		w.WriteHeader(http.StatusOK)
	}), s.LoadSession, s.CSRF)

	req := httptest.NewRequest(http.MethodGet, "/app/profile", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: sess.ID})
	chain.ServeHTTP(httptest.NewRecorder(), req)

	if got != sess.CSRF {
		t.Errorf("template token = %q, want the session token", got)
	}
}
