package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// The screen under a sheet (the owner's decision of 15.10.2026). A sheet
// opened by its address without scripts is a page, and its frames draw it
// over the screen it was opened from (s-contacts-2: the combined book under
// «Move contact»; Typecase's Overlay: «under the scrim, the same body of the
// shell»). The sheet's answer then carries that screen (view.Under): the
// page its back names — or the page of the record or list the sheet belongs
// to —, as its own address answers it, the same handler asked the same way.
// A sheet asked for as a fragment carries none: the screen stays with the
// client (C-7.2).

// underKey marks a request asked only for its page: writePage catches the
// page instead of writing it.
type underKey struct{}

// caughtPage is the page a request asked for its page answered.
type caughtPage struct {
	body   any
	status int
}

// catchPage reports whether r is asked only for its page, and keeps it.
func catchPage(r *http.Request, status int, body any) bool {
	c, ok := r.Context().Value(underKey{}).(*caughtPage)
	if ok {
		c.body, c.status = body, status
	}
	return ok
}

// discardWriter is the answer of a request asked only for its page: what
// is written to it — a redirect, an error — is dropped.
type discardWriter struct{ h http.Header }

func (d *discardWriter) Header() http.Header {
	if d.h == nil {
		d.h = http.Header{}
	}
	return d.h
}
func (d *discardWriter) Write(b []byte) (int, error) { return len(b), nil }
func (d *discardWriter) WriteHeader(int)             {}

// underOf is the screen under a sheet r answers as a page: the page at back
// when it is an address of the app, else at fallback; nil for a fragment, an
// address that answers no page of view.Under, or one that fails.
func (s *Server) underOf(r *http.Request, back, fallback string) *view.Under {
	if render.WantsFragment(r) {
		return nil
	}
	path := s.sheetBack(r, back, false)
	if path == "" {
		path = fallback
	}
	u, err := url.Parse(path)
	if err != nil {
		return nil
	}
	caught := &caughtPage{}
	sub := r.Clone(context.WithValue(r.Context(), underKey{}, caught))
	sub.Method, sub.URL, sub.RequestURI, sub.Body, sub.Form, sub.PostForm = http.MethodGet, u, u.RequestURI(), http.NoBody, nil, nil
	sub.Header.Del(render.HeaderView)
	sub.Header.Del(render.HeaderTarget)
	h, _ := s.mux.Handler(sub)
	h.ServeHTTP(&discardWriter{}, sub)
	if caught.status != http.StatusOK {
		return nil
	}
	return underPage(caught.body)
}

// sheetBack is the page a sheet was opened from, to stand over and to go
// back to with Cancel (Лс30, Лс31): back, else the page the link was
// followed from when referer (its Referer, as the password's sheet reads
// it: a sheet opened by a link — not one that answers a form, whose Referer
// is the form); "" — none of this instance's pages. A path of the app
// without the base path is taken as back too.
func (s *Server) sheetBack(r *http.Request, back string, referer bool) string {
	if p, ok := s.returnPath(r, back); ok {
		return p
	}
	if p := strings.TrimPrefix(back, s.basePath); strings.HasPrefix(p, "/app/") && !strings.Contains(p, "//") && !strings.Contains(p, "..") && !strings.Contains(p, "\\") {
		return p
	}
	if p, ok := s.returnPath(r, r.Referer()); ok && referer {
		return p
	}
	return ""
}

// cancelTo sends a sheet's Cancel (id) to the page it was opened from, when
// that is known (Лс30): the screen under it, not the record's page.
func (s *Server) cancelTo(r *http.Request, back string, actions []view.Action, id string) {
	p := s.sheetBack(r, back, false)
	if p == "" {
		return
	}
	for i := range actions {
		if actions[i].ID == id {
			actions[i].Href = s.route(p)
		}
	}
}

// underPage is a page as the screen under a sheet; nil — a page no sheet is
// drawn over.
func underPage(body any) *view.Under {
	switch p := body.(type) {
	case view.ContactsListPage:
		return &view.Under{Contacts: &p}
	case view.ContactPage:
		return &view.Under{Contact: &p}
	case view.AgendaPage:
		return &view.Under{Agenda: &p}
	case view.EventPage:
		return &view.Under{Event: &p}
	case view.TasksPage:
		return &view.Under{Tasks: &p}
	case view.TaskPage:
		return &view.Under{Task: &p}
	case view.NotesPage:
		return &view.Under{Notes: &p}
	case view.NotePage:
		return &view.Under{Note: &p}
	case view.ConnectionsPage:
		return &view.Under{Connections: &p}
	}
	return nil
}
