package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/antongulenko/http-isolation-proxy/services"
	"github.com/gorilla/mux"
)

const (
	order_processing_buf     = 50
	order_processing_workers = 10
)

func launchOrderProcessing(shop *Shop) {
	ids := make(chan string, order_processing_buf)
	go shop.LoopScanOrders(ids)
	for i := 0; i < order_processing_workers; i++ {
		go shop.LoopProcessOrders(ids)
	}
}

func main() {
	addr := flag.String("listen", "0.0.0.0:9004", "Endpoint address")
	redisEndpoint := flag.String("redis", "127.0.0.1:6379", "Redis endpoint")
	paymentEndpoint := flag.String("payment", "localhost:9002", "Endpoint for payment service")
	catalogEndpoint := flag.String("catalog", "localhost:9003", "Endpoint for catalog service")
	services.ParseBalanceEndpointsFlags()
	flag.Parse()
	services.ParseLoadBalanceConfig()
	services.EnableResponseLogging()
	services.ConfigureOpenFilesLimit()

	redisClient, err := services.ConnectRedis(*redisEndpoint)
	if err != nil {
		log.Fatalln(err)
	}

	if err := services.RegisterLockScripts(redisClient); err != nil {
		log.Fatalln("Failed to register redis scripts", err)
	}

	shop := &Shop{
		redis:           redisClient,
		redisLockValue:  *addr, // Should be unique and constant per endpoint
		catalogEndpoint: *catalogEndpoint,
		paymentEndpoint: *paymentEndpoint,
	}
	launchOrderProcessing(shop)

	mux := mux.NewRouter()
	mux.HandleFunc("/shop", shop.show_items).Methods("GET")
	mux.HandleFunc("/order", shop.order_item).Methods("POST").MatcherFunc(services.MatchFormKeys("user", "item", "qty"))
	mux.HandleFunc("/order/{order}", shop.get_order).Methods("GET")
	mux.HandleFunc("/orders/{user}", shop.show_orders).Methods("GET")

	services.L.Warnf("Running on " + *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
