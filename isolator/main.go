package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/antongulenko/http-isolation-proxy/proxy"
	"github.com/antongulenko/http-isolation-proxy/services"
)

const (
	stats_addr = ":9006"
)

func check(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func handle(p *proxy.IsolationProxy, name string, addr string) {
	check(p.Handle(name, addr))
}

func main() {
	flag.Parse()
	check(services.SetOpenFilesLimit(40000))

	reg := make(proxy.LocalRegistry)
	service := func(name string, endpoints ...string) {
		for _, addr := range endpoints {
			endpoint := &proxy.Endpoint{
				Service: name,
				Host:    addr,
			}
			endpoint.TestActive()
			reg.Add(name, endpoint)
		}
	}

	service("payment", "localhost:8000", "localhost:8001", "localhost:8002")
	service("shop", "localhost:8003", "localhost:8004", "localhost:8005")
	service("catalog", "localhost:8006", "localhost:8007", "localhost:8008")
	service("bank", "localhost:8009")

	p := &proxy.IsolationProxy{
		Registry: reg,
	}
	services.EnableResponseLogging()
	p.ServeStats("/stats")
	proxy.ServeRuntimeStats("/runtime")

	go handle(p, "bank", "localhost:9001")
	go handle(p, "payment", "localhost:9002")
	go handle(p, "catalog", "localhost:9003")
	go handle(p, "shop", "localhost:9004")
	go handle(p, "test", "localhost:9005") // No backends
	check(http.ListenAndServe(stats_addr, nil))
}
