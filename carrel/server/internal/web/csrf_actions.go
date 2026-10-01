package web

import (
	"net/http"
	"reflect"

	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// stampingRenderer gives every POST action of what it renders the token of
// the request's session (C-11, Action.CSRF): a client without scripts sends
// a POST outside a form as a form of its own, with that token as _csrf. One
// place for every action the handlers build, and nothing else of the view
// model changes. A request without a session — the pages outside the
// shell, the public ones — stamps nothing: their forms carry their own.
type stampingRenderer struct {
	render.Renderer
	token func(r *http.Request) string
}

func (sr stampingRenderer) Page(w http.ResponseWriter, r *http.Request, status int, body any) error {
	return sr.Renderer.Page(w, r, status, stampCSRF(body, sr.token(r)))
}

func (sr stampingRenderer) Fragment(w http.ResponseWriter, r *http.Request, status int, target string, body any, extra []render.OOB) error {
	token := sr.token(r)
	for i := range extra {
		extra[i].Body = stampCSRF(extra[i].Body, token)
	}
	return sr.Renderer.Fragment(w, r, status, target, stampCSRF(body, token), extra)
}

// sessionToken is the CSRF token of the request's session; empty without
// one.
func (s *Server) sessionToken(r *http.Request) string {
	if sess, ok := s.sessionFrom(r); ok {
		return sess.CSRFToken()
	}
	return ""
}

var actionType = reflect.TypeOf(view.Action{})

// stampCSRF returns body with token in every POST Action, the items of an
// arrow and of a menu among them. The body is copied, never changed in
// place: a handler may keep what it rendered.
func stampCSRF(body any, token string) any {
	if token == "" || body == nil {
		return body
	}
	v := reflect.ValueOf(body)
	out := reflect.New(v.Type()).Elem()
	out.Set(stampValue(v, token))
	return out.Interface()
}

func stampValue(v reflect.Value, token string) reflect.Value {
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
		if v.Type() == actionType && fresh.FieldByName("Method").String() == http.MethodPost {
			fresh.FieldByName("CSRF").SetString(token)
		}
		return fresh
	}
	return v
}
