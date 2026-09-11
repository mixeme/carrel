package web

import "strings"

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
