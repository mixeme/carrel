package web

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/render"
)

type handler func(*Server, http.ResponseWriter, *http.Request)

func (s *Server) handle(pattern string, h handler) {
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		s.serve(w, r, h)
	})
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request, h handler) {
	setSecretPathHeaders(w, r)
	now := time.Now()

	// /healthz and /about carry no bucket at all (architecture.md): probes
	// and the licence obligation never see 429, cookie or not.
	if c, err := r.Cookie(cookieName); err == nil && c.Value != "" && !unmetered(r.URL.Path) {
		if !s.limit.allowSession(c.Value, now) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		sess, hasSess := s.sessionFrom(r)
		// A file upload has no ceiling of Carrel's (C-12): its body is read as it
		// arrives, and only the fields in front of the files are bounded.
		stream := hasSess && isFileUpload(r)
		if !stream || strings.HasPrefix(r.URL.Path, "/app/contacts/") {
			r.Body = http.MaxBytesReader(w, r.Body, s.bodyLimit(r))
		}

		if needsCSRF(r) && hasSess {
			header := r.Header.Get(csrfHeaderName)
			if header != "" && !sess.CheckCSRF(header) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			if stream {
				// _csrf comes before the first file: a token after it cannot be
				// read without buffering the file. With the token in the header
				// the fields are read here all the same: a body without a
				// ceiling is never handed on unread, where a ParseMultipartForm
				// would put its files on disk (audit №4, А4-1).
				up, err := readUploadFields(r)
				switch {
				case err != nil && tooLarge(err):
					http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
					return
				case err != nil:
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				case header == "" && !sess.CheckCSRF(up.fields.Get(csrfFormField)):
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
				r = r.WithContext(context.WithValue(r.Context(), uploadKey{}, up))
			} else if header == "" {
				// With the token in the header the body is left to the
				// handler (readForm), within the ceiling set above.
				if token, err := csrfTokenFromBody(r); err != nil {
					if tooLarge(err) {
						http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
						return
					}
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				} else if !sess.CheckCSRF(token) {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
			}
		} else if needsCSRF(r) && !hasSess && needsAnonymousCSRF(r) {
			if token := r.Header.Get(csrfHeaderName); token != "" {
				if !checkAnonymousCSRF(r, token) {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
			} else if token, err := csrfTokenFromBody(r); err != nil {
				if tooLarge(err) {
					http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
					return
				}
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			} else if !checkAnonymousCSRF(r, token) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		} else if err := r.ParseForm(); err != nil {
			if tooLarge(err) {
				http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
	}

	h(s, w, r)
}

// bodyLimit is the cap of a request body before anything reads it: 1 MiB for
// a form; for a contact photo upload, CARREL_PHOTO_MAX_UPLOAD_BYTES with a
// form's worth for the fields around the file (security.md, CSRF). The _csrf
// of such an upload is read within this cap too. A file to import has no
// cap (Р-73): it comes as a stream (isFileUpload).
func (s *Server) bodyLimit(r *http.Request) int64 {
	seg := strings.Split(r.URL.EscapedPath(), "/")
	switch {
	case len(seg) >= 6 && seg[1] == "app" && seg[2] == "contacts" &&
		(len(seg) == 6 && seg[5] == "new" || len(seg) == 7 && seg[6] == "edit"):
		return s.cfg.PhotoMaxUploadBytes + conf.MaxFormBodyBytes
	case len(seg) == 7 && seg[1] == "app" && seg[2] == "contacts" && seg[6] == "photo-preview":
		return s.cfg.PhotoMaxUploadBytes + conf.MaxFormBodyBytes
	}
	return conf.MaxFormBodyBytes
}

func unmetered(path string) bool {
	return path == "/healthz" || path == "/about"
}

func tooLarge(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

func needsCSRF(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func (s *Server) writePage(w http.ResponseWriter, r *http.Request, status int, body any) {
	if catchPage(r, status, body) {
		return // asked only for its page: the screen under a sheet (under.go)
	}
	if render.WantsFragment(r) {
		var extra []render.OOB
		// A list's row asks its panel (?open=, the live layer): the list
		// comes with it, its open row lit — the row lit stayed the one
		// opened before (24.11.2026).
		if render.FragmentTarget(r) == render.TargetDetails && panelLists[reflect.TypeOf(body)] {
			if open, _ := r.Context().Value(openKey{}).(string); open != "" {
				extra = []render.OOB{{Target: render.TargetMain, Body: body}}
			}
		}
		_ = s.render.Fragment(w, r, status, render.FragmentTarget(r), body, extra)
		return
	}
	_ = s.render.Page(w, r, status, onPaper(r, body))
}

// onPaper is a page asked on paper (?print=1, U-10) with its print header
// saying so (Print.Paper): the same route and the same model, which the
// client draws as a page on paper. A page without a shell, or one asked
// otherwise, is as it was.
func onPaper(r *http.Request, body any) any {
	if r.URL.Query().Get(render.PrintParam) != render.PrintValue {
		return body
	}
	v := reflect.ValueOf(body)
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return body
	}
	copied := reflect.New(v.Type()).Elem()
	copied.Set(v)
	paper := copied
	for _, name := range []string{"Shell", "Print", "Paper"} {
		if paper.Kind() != reflect.Struct {
			return body
		}
		if paper = paper.FieldByName(name); !paper.IsValid() {
			return body
		}
	}
	if !paper.CanSet() || paper.Kind() != reflect.Bool {
		return body
	}
	paper.SetBool(true)
	return copied.Interface()
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func (s *Server) redirect(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, s.route(url), http.StatusSeeOther)
}
