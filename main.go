package main

import (
	"log"
	"net/http"
)

func myHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func main() {
	const port = "8080"

	mux := http.NewServeMux()

	var srv http.Server
	srv.Addr = ":" + port
	srv.Handler = mux

	mux.Handle("/app/", http.StripPrefix("/app", http.FileServer(http.Dir("."))))
	mux.HandleFunc("/healthz", myHandler)

	log.Printf("Serving on port: %s\n", port)
	log.Fatal(srv.ListenAndServe())
}
