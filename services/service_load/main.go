package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/antongulenko/golib"
	"github.com/antongulenko/http-isolation-proxy/services"
)

func main() {
	var shops golib.StringSlice
	parallel_orders := flag.Uint("orders", 20, "Number of open orders running at the same time")
	bank := flag.String("bank", "localhost:9001", "Bank endpoint")
	timeout := flag.Duration("timeout", 0, "Timeout for automatically stopping load generation")
	flag.Var(&shops, "shop", "Shop endpoint(s)")
	flag.Parse()
	if len(shops) == 0 {
		log.Fatalln("Specify at least one -shop")
	}

	golib.ConfigureOpenFilesLimit()

	orders := OrderPool{
		ParallelOrders: *parallel_orders,
		Bank:           *bank,
		Shops:          shops,
		User:           "Load",
	}
	orders.Start()

	onInterrupt(orders.Terminate)
	if *timeout > 0 {
		services.L.Warnf("Terminating automatically after %v", timeout)
		time.AfterFunc(*timeout,
			func() {
				services.L.Warnf("Timer of %v expired. Terminating...", timeout)
				orders.Terminate()
			})
	}
	orders.Wait()
	orders.PrintStats()
}

func onInterrupt(handler func()) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	go func() {
		defer signal.Stop(interrupt)
		<-interrupt
		handler()
	}()
}
