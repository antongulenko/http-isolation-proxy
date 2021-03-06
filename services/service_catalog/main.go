package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/antongulenko/golib"
	"github.com/antongulenko/http-isolation-proxy/services"
	"github.com/gorilla/mux"
)

const (
	refill_timeout = 5 * time.Minute
)

func fillDefaultCatalog(catalog *Catalog) error {
	return catalog.ensureItems([]*Item{
		catalog.MakeItem("DVD", 5000, 4.99),
		catalog.MakeItem("Toaster", 1000, 12.99),
		catalog.MakeItem("Laptop", 500, 499.00),
		catalog.MakeItem("TV", 100, 1099.00),
		catalog.MakeItem("Spaceship", 0, 5000000000),
	})
}

func main() {
	addr := flag.String("listen", "0.0.0.0:9003", "Endpoint address")
	redisEndpoint := flag.String("redis", "127.0.0.1:6379", "Redis endpoint")
	services.ParseBalanceEndpointsFlags()
	flag.Parse()
	services.ParseLoadBalanceConfig()
	services.EnableResponseLogging()
	golib.ConfigureOpenFilesLimit()

	redisClient, err := services.ConnectRedis(*redisEndpoint)
	if err != nil {
		log.Fatalln(err)
	}

	if err := services.RegisterLockScripts(redisClient); err != nil {
		log.Fatalln("Failed to register redis scripts", err)
	}

	catalog := &Catalog{
		redis:          redisClient,
		redisLockValue: services.EndpointLockValue(*addr),
	}
	if err := fillDefaultCatalog(catalog); err != nil {
		log.Fatalln("Error filling default catalog:", err)
	}
	go catalog.RefillItems(refill_timeout, map[string]uint64{
		"DVD":       500,
		"Toaster":   100,
		"Laptop":    50,
		"TV":        10,
		"Spaceship": 1,
	})

	mux := mux.NewRouter()
	mux.HandleFunc("/items", catalog.show_items).Methods("GET")
	mux.HandleFunc("/item/{name}", catalog.show_item).Methods("GET")
	mux.HandleFunc("/item/{name}/ship", catalog.ship_item).Methods("POST").MatcherFunc(services.MatchFormKeys("user", "qty", "ts"))
	mux.HandleFunc("/shipment/{id}", catalog.show_shipment).Methods("GET")
	mux.HandleFunc("/shipment/{id}/commit", catalog.commit_shipment).Methods("POST")
	mux.HandleFunc("/shipment/{id}/cancel", catalog.cancel_shipment).Methods("POST")
	mux.HandleFunc("/shipment/{id}/deliver", catalog.deliver_shipment).Methods("POST")
	services.L.Warnf("Running on " + *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
