package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/antongulenko/golib"
	"github.com/antongulenko/http-isolation-proxy/services"
	"github.com/antongulenko/http-isolation-proxy/services/service_bank/bankApi"
	"github.com/gorilla/mux"
)

func main() {
	addr := flag.String("listen", "0.0.0.0:9002", "Endpoint address")
	redisEndpoint := flag.String("redis", "127.0.0.1:6379", "Redis endpoint")
	bankEndpoint := flag.String("bank", "localhost:9001", "Endpoint for bank service")
	services.ParseBalanceEndpointsFlags()
	flag.Parse()
	services.ParseLoadBalanceConfig()
	services.EnableResponseLogging()
	golib.ConfigureOpenFilesLimit()

	bank := bankApi.NewHttpBank(*bankEndpoint)
	redisClient, err := services.ConnectRedis(*redisEndpoint)
	if err != nil {
		log.Fatalln(err)
	}

	if err := services.RegisterLockScripts(redisClient); err != nil {
		log.Fatalln("Failed to register redis scripts", err)
	}

	payments := &Payments{
		bank:           bank,
		redis:          redisClient,
		accountName:    "store",
		redisLockValue: services.EndpointLockValue(*addr),
	}
	mux := mux.NewRouter()
	mux.HandleFunc("/payment", payments.new_payment).Methods("POST").MatcherFunc(services.MatchFormKeys("user", "value", "ts"))
	mux.HandleFunc("/payment/{id}", payments.show_payment).Methods("GET")
	mux.HandleFunc("/payment/{id}/commit", payments.commit_payment).Methods("POST")
	mux.HandleFunc("/payment/{id}/cancel", payments.cancel_payment).Methods("POST")

	services.L.Warnf("Running on " + *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
