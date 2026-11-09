package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// C-14.2: a new form whose files or photo the session no longer keeps, its
// Save refused (Typecase's note 181 p. 2, task 256): the event «Планёрка по
// обложке» in «Личный» with two files that four newer forms pushed out
// (s-agenda-32), and the card «Ирина Мельникова» whose photo the photo of
// another new card took the place of (s-contacts-38).
func TestFixtureNewFormLost(t *testing.T) {
	app := newMockupApp(t)
	// Read as for the new event's form of s-agenda-15: the calendars and
	// the books, so its suggestions are the same.
	combined(t, app.h, app.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	app.h.do(get(app.listPath(t, "calendar", "lichnyj")+"?from=2026-08-10", app.sess))
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	t.Run("app-event-new/files-gone", func(t *testing.T) {
		list := app.listPath(t, "calendar", "lichnyj")
		attach := func(typed url.Values, names ...string) url.Values {
			t.Helper()
			page := decode[view.EventEditPage](t, app.h.do(get(list+"/new", app.sess)))
			values := formFields(page.Form)
			for k, v := range typed {
				values[k] = v
			}
			for _, name := range names {
				resp := app.h.do(submitWithFile(actionByID(page.Form.Actions, "attach.add").Href, app.sess, values, "file", name, []byte("a stub of "+name)))
				back := resp.Header().Get("Location")
				if resp.Code != http.StatusSeeOther || !strings.Contains(back, "uid=") {
					t.Fatalf("attach %s: %d %s", name, resp.Code, back)
				}
				page = decode[view.EventEditPage](t, app.h.do(get(back, app.sess)))
				values = formFields(page.Form)
			}
			return values
		}
		first := attach(url.Values{"summary": {"Планёрка по обложке"}, "location": {"Переговорная 1"},
			"start.date": {"2026-08-13"}, "start.time": {"14:00"}, "end.date": {"2026-08-13"}, "end.time": {"15:00"}},
			"макет-обложки-v3.pdf", "шрифты-обложки.png")
		for _, name := range []string{"a.pdf", "b.pdf", "c.pdf", "d.pdf"} {
			attach(url.Values{"summary": {"Другая форма"}}, name)
		}
		app.compare(t, "app-event-new/files-gone.json", app.h.do(postForm(list+"/new", app.sess, first)))
	})
	t.Run("app-contact-new/photo-gone", func(t *testing.T) {
		book := app.listPath(t, "contacts", "kontakty")
		photo := func(name string) url.Values {
			t.Helper()
			fresh := decode[view.ContactEditPage](t, app.h.do(get(book+"/new", app.sess)))
			resp := app.h.do(uploadRequest(actionHref(t, fresh.Photo.Actions, "photo.add"), app.sess, true, name, jpegOf(t, halvesImage(400, 400))))
			preview := resp.Header().Get("Location")
			page := decode[view.PhotoPreviewPage](t, app.h.do(get(preview, app.sess)))
			resp = app.h.do(postForm(actionHref(t, page.Actions, "photo.save"), app.sess, cropForm(page)))
			if resp.Code != http.StatusSeeOther {
				t.Fatalf("save photo %s: %d", name, resp.Code)
			}
			return formFields(decode[view.ContactEditPage](t, app.h.do(get(resp.Header().Get("Location"), app.sess))).Form)
		}
		first := photo("IMG_0101.jpg")
		photo("IMG_0102.jpg")
		first.Set("fn", "Ирина Мельникова")
		first.Set("n.given", "Ирина")
		first.Set("n.family", "Мельникова")
		first.Set("tel.0", "+7 916 204-37-15")
		app.compare(t, "app-contact-new/photo-gone.json", app.h.do(postForm(book+"/new", app.sess, first)))
	})
}
