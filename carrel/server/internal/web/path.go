package web

import (
	"net/url"
	"strings"
)

// route prefixes an application path with CARREL_BASE_PATH. Absolute URLs
// and empty paths are unchanged.
func (s *Server) route(path string) string {
	if path == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	bp := s.basePath
	if bp == "" {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return bp + path
}

func (s *Server) cookiePath() string {
	if s.basePath == "" {
		return "/"
	}
	return s.basePath
}

// postQuery is an Action.Href for a POST: the route plus the fields the
// handler reads via FormValue. Action has no body in C-6, so the fields
// travel on the query string.
func (s *Server) postQuery(path string, fields url.Values) string {
	p := s.route(path)
	enc := fields.Encode()
	if enc == "" {
		return p
	}
	return p + "?" + enc
}
