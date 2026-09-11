package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"carrel.dev/typecase"
	"carrel.dev/typecase/catalog"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	css, err := typecase.CSS()
	if err != nil {
		log.Fatal(err)
	}
	themeJS, err := typecase.ThemeJS()
	if err != nil {
		log.Fatal(err)
	}
	js, err := typecase.JS()
	if err != nil {
		log.Fatal(err)
	}
	entries := catalog.Entries()
	withJS := catalog.PageHTML("Typecase catalog", entries, true)
	withoutJS := catalog.PageHTML("Typecase catalog (no behavior)", entries, false)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(withJS))
	})
	mux.HandleFunc("/static", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(withoutJS))
	})
	mux.HandleFunc("/typecase.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write([]byte(css))
	})
	mux.HandleFunc("/theme.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write(themeJS)
	})
	mux.HandleFunc("/tc.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write(js)
	})

	addr := ":" + port
	fmt.Fprintf(os.Stderr, "catalog listening on http://localhost%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
