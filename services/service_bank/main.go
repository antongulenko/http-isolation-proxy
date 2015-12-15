package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/antongulenko/http-isolation-proxy/services"
	"github.com/gorilla/mux"
)

func main() {
	addr := flag.String("listen", "0.0.0.0:9001", "Endpoint address")
	flag.Parse()
	store := NewAccountStore()
	for i := 0; i < 20; i++ {
		go store.HandleTransactions()
	}

	EnableTransactionLogging()
	services.EnableResponseLogging()

	mux := mux.NewRouter()
	mux.HandleFunc("/account/{id}", store.show_account).Methods("GET")
	mux.HandleFunc("/account/{id}/deposit", store.handle_deposit).Methods("POST").MatcherFunc(services.MatchFormKeys("value"))
	mux.HandleFunc("/account/{id}/transfer", store.handle_transfer).Methods("POST").MatcherFunc(services.MatchFormKeys("value", "target"))
	mux.HandleFunc("/transaction/{id}", store.show_transaction).Methods("GET")
	mux.HandleFunc("/transaction/{id}/cancel", store.cancel_transaction).Methods("POST")
	mux.HandleFunc("/transaction/{id}/revert", store.revert_transaction).Methods("POST")
	mux.HandleFunc("/transaction/{id}/commit", store.commit_transaction).Methods("POST")

	log.Println("Running on " + *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
