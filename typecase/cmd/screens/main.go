package main

import (
	"log"
	"net/http"
	"os"

	"carrel.dev/typecase/screens"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	mux, err := screens.Mux("")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("typecase screens on http://localhost:%s/", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
