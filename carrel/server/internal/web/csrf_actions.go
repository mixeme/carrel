package web

import (
	"net/http"
	"reflect"
	"strings"

	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// stampingRenderer gives every POST action of what it renders the token of
// the request's session (C-11, Action.CSRF): a client without scripts sends
// a POST outside a form as a form of its own, with that token as _csrf. A
// request without a session — the pages outside the shell, the public ones
// — stamps no token: their forms carry their own. And every action and
// form gets the zone of C-7.2 a fragment answer of its address replaces
// (Action.Target, Form.Target; targets.go), by C-5. One place for every
// action the handlers build, and nothing else of the view model changes.
type stampingRenderer struct {
	render.Renderer
	token func(r *http.Request) string
	base  string // CARREL_BASE_PATH, normalised: the addresses carry it
}

func (sr stampingRenderer) Page(w http.ResponseWriter, r *http.Request, status int, body any) error {
	look := lookOf(r)
	return sr.Renderer.Page(w, r, status, stampPanel(r, stampLook(stampView(body, sr.token(r), sr.base), look), look))
}

func (sr stampingRenderer) Fragment(w http.ResponseWriter, r *http.Request, status int, target string, body any, extra []render.OOB) error {
	token := sr.token(r)
	for i := range extra {
		extra[i].Body = stampLook(stampView(extra[i].Body, token, sr.base), lookOf(r))
	}
	look := lookOf(r)
	return sr.Renderer.Fragment(w, r, status, target, stampPanel(r, stampLook(stampView(body, token, sr.base), look), look), extra)
}

// sessionToken is the CSRF token of the request's session; empty without
// one.
func (s *Server) sessionToken(r *http.Request) string {
	if sess, ok := s.sessionFrom(r); ok {
		return sess.CSRFToken()
	}
	return ""
}

var (
	actionType = reflect.TypeOf(view.Action{})
	formType   = reflect.TypeOf(view.Form{})
	shellType  = reflect.TypeOf(view.Shell{})
)

// stampView returns body with token in every POST Action, the items of an arrow
// and of a menu among them, and with the target of C-7.2 in every Action
// and Form that has none. The body is copied, never changed in place: a
// handler may keep what it rendered.
func stampView(body any, token, base string) any {
	if body == nil {
		return body
	}
	v := reflect.ValueOf(body)
	out := reflect.New(v.Type()).Elem()
	out.Set(stampValue(v, stamper{token: token, base: base}))
	return out.Interface()
}

// stamper is what stampView puts in: the session's token, and the base path the
// addresses are read under.
type stamper struct{ token, base string }

func stampValue(v reflect.Value, token stamper) reflect.Value {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		p := reflect.New(v.Type().Elem())
		p.Elem().Set(stampValue(v.Elem(), token))
		return p
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		out := reflect.New(v.Type()).Elem()
		out.Set(stampValue(v.Elem(), token))
		return out
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		fresh := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			fresh.Index(i).Set(stampValue(v.Index(i), token))
		}
		return fresh
	case reflect.Array:
		fresh := reflect.New(v.Type()).Elem()
		for i := 0; i < v.Len(); i++ {
			fresh.Index(i).Set(stampValue(v.Index(i), token))
		}
		return fresh
	case reflect.Map:
		if v.IsNil() {
			return v
		}
		fresh := reflect.MakeMapWithSize(v.Type(), v.Len())
		for it := v.MapRange(); it.Next(); {
			fresh.SetMapIndex(it.Key(), stampValue(it.Value(), token))
		}
		return fresh
	case reflect.Struct:
		fresh := reflect.New(v.Type()).Elem()
		fresh.Set(v)
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				fresh.Field(i).Set(stampValue(v.Field(i), token))
			}
		}
		switch v.Type() {
		case actionType:
			method := fresh.FieldByName("Method").String()
			if method == http.MethodPost && token.token != "" {
				fresh.FieldByName("CSRF").SetString(token.token)
			}
			if t := fresh.FieldByName("Target"); t.String() == "" {
				t.SetString(answerZone(token.base, method, fresh.FieldByName("Href").String()))
			}
		case formType:
			if t := fresh.FieldByName("Target"); t.String() == "" {
				t.SetString(answerZone(token.base, strings.ToUpper(fresh.FieldByName("Method").String()), fresh.FieldByName("Action").String()))
			}
		}
		return fresh
	}
	return v
}

// stampLook gives every shell of what is drawn this browser's Look
// (Appearance, appearance-1): the page and the screen under a sheet alike.
// stampView has already made the value its own, so it is changed in place.
func stampLook(body any, look view.Look) any {
	if p, ok := body.(view.NotePage); ok && len(p.Doc.Controls) > 0 {
		look.Wide = p.Look.Wide
	}
	if body == nil {
		return body
	}
	v := reflect.ValueOf(body)
	out := reflect.New(v.Type()).Elem()
	out.Set(v)
	putLook(out, look)
	return out.Interface()
}

func putLook(v reflect.Value, look view.Look) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			if v.Kind() == reflect.Interface {
				inner := reflect.New(v.Elem().Type()).Elem()
				inner.Set(v.Elem())
				putLook(inner, look)
				if v.CanSet() {
					v.Set(inner)
				}
				return
			}
			putLook(v.Elem(), look)
		}
	case reflect.Struct:
		if v.Type() == shellType {
			v.FieldByName("Look").Set(reflect.ValueOf(look))
			return
		}
		for i := 0; i < v.NumField(); i++ {
			if f := v.Field(i); v.Type().Field(i).IsExported() && f.CanSet() {
				switch f.Kind() {
				case reflect.Struct, reflect.Pointer, reflect.Interface:
					putLook(f, look)
				}
			}
		}
	}
}
