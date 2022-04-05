package test

import (
	"log"
	"net/http"
	"testing"

	"github.com/gorilla/mux"
)

func TestVNCServer(t *testing.T) {
	log.Println("empty")
}

func TestVNCClient(t *testing.T) {
	r := mux.NewRouter()
	s := http.StripPrefix("/", http.FileServer(http.Dir("../http")))
	r.PathPrefix("/").Handler(s)
	http.Handle("/", r)
	log.Fatal(http.ListenAndServe(":80", nil))
}
