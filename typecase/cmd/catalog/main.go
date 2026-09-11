package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"carrel.dev/typecase/catalog"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	mux, err := catalog.Mux("")
	if err != nil {
		log.Fatal(err)
	}
	addr := ":" + port
	fmt.Fprintf(os.Stderr, "catalog listening on http://localhost%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
