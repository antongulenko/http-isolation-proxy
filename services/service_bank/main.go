package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/antongulenko/http-isolation-proxy/services"
	"github.com/gorilla/mux"
)

func main() {
	services.ConfiguredOpenFilesLimit = 40000
	addr := flag.String("listen", "0.0.0.0:9001", "Endpoint address")
	flag.Parse()
	services.EnableResponseLogging()
	services.ConfigureOpenFilesLimit()

	store := NewAccountStore(1000, 200)

	mux := mux.NewRouter()
	mux.HandleFunc("/stats", store.show_stats).Methods("GET")
	mux.HandleFunc("/account/{id}", store.show_account).Methods("GET")
	mux.HandleFunc("/account/{id}/deposit", store.handle_deposit).Methods("POST").MatcherFunc(services.MatchFormKeys("value"))
	mux.HandleFunc("/account/{id}/transfer", store.handle_transfer).Methods("POST").MatcherFunc(services.MatchFormKeys("value", "target"))
	mux.HandleFunc("/transaction/{id}", store.show_transaction).Methods("GET")
	mux.HandleFunc("/transaction/{id}/cancel", store.cancel_transaction).Methods("POST")
	mux.HandleFunc("/transaction/{id}/revert", store.revert_transaction).Methods("POST")
	mux.HandleFunc("/transaction/{id}/commit", store.commit_transaction).Methods("POST")

	services.L.Warnf("Running on " + *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
