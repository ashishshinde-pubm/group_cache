package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func main() {
	// Setting up a simple HTTP server to handle requests
	router := mux.NewRouter()
	router.HandleFunc("/{key}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		key := vars["key"]
		value := key + "_value"
		fmt.Println("success fetch value = ", value, " for source - ", r.Header["Source"])
		fmt.Fprintf(w, value)
	}).Methods("GET")

	log.Fatal(http.ListenAndServe("localhost:5001", router))
}
