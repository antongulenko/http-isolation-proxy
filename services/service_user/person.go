package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/antongulenko/http-isolation-proxy/services"
	"github.com/antongulenko/http-isolation-proxy/services/service_bank/bankApi"
	"github.com/antongulenko/http-isolation-proxy/services/service_shop/shopApi"
)

const (
	min_sleep = 3000
	max_sleep = 5000
	min_pay   = 800
	max_pay   = 5000

	start_day      = 8
	days_per_month = 10
)

type Person struct {
	sleepTime time.Duration

	bank          bankApi.Bank
	shopEndpoints []string

	monthlyPay float64
	Name       string

	ShopRequests    uint64
	BankRequests    uint64
	TotalErrors     uint64
	SkippedShopping uint64

	running bool
	paused  bool
	cond    sync.Cond

	OpenOrdersLimit uint
	openOrders      map[string]bool
}

func (person *Person) String() string {
	return fmt.Sprintf("%v: sleeps %v, earns %v", person.Name, person.sleepTime, person.monthlyPay)
}

func RandomPerson(name string, bankEndpoint string, shopEndpoints []string) *Person {
	sleepTime := (time.Duration(rand.Int63n(max_sleep-min_sleep) + min_sleep)) * time.Millisecond
	monthlyPay := rand.Float64()*max_pay + min_pay

	return &Person{
		Name:          name,
		sleepTime:     sleepTime,
		monthlyPay:    monthlyPay,
		bank:          bankApi.NewHttpBank(bankEndpoint),
		shopEndpoints: shopEndpoints,
		cond:          sync.Cond{L: new(sync.Mutex)},
		paused:        true,
		running:       true,
		openOrders:    make(map[string]bool),
	}
}

func (person *Person) Live(wg *sync.WaitGroup) {
	wg.Add(1)
	go person.doLive(wg)
}

func (person *Person) Start() {
	person.cond.L.Lock()
	defer person.cond.L.Unlock()
	person.paused = false
	person.cond.Broadcast()
}

func (person *Person) Pause() {
	person.cond.L.Lock()
	defer person.cond.L.Unlock()
	person.paused = true
}

func (person *Person) Terminate() {
	person.cond.L.Lock()
	defer person.cond.L.Unlock()
	person.paused = false
	person.running = false
	person.cond.Broadcast()
}

func (person *Person) pauseOrQuit() bool {
	person.cond.L.Lock()
	defer person.cond.L.Unlock()
	for person.paused {
		person.cond.Wait()
	}
	return !person.running
}

func (person *Person) doLive(wg *sync.WaitGroup) {
	defer wg.Done()
	day := start_day
	for {
		if person.pauseOrQuit() {
			return
		}
		day++
		if day%days_per_month == 0 {
			person.earn()
		}
		if person.pauseOrQuit() {
			return
		}
		person.shop()
		if person.pauseOrQuit() {
			return
		}
		person.sleep()
	}
}

func (person *Person) error(err error) bool {
	if err == nil {
		return false
	} else {
		person.TotalErrors++
		services.L.Warnf("%v: %v", person.Name, err)
		return true
	}
}

func (person *Person) earn() {
	_, err := person.bank.Deposit(person.Name, person.monthlyPay)
	person.BankRequests++
	services.L.Logf("%v earning %v", person.Name, person.monthlyPay)
	person.error(err)
}

func (person *Person) pickShop() string {
	num := rand.Int31n(int32(len(person.shopEndpoints)))
	endpoint := person.shopEndpoints[num]
	services.L.Tracef("Picked shop endpoint: %v", endpoint)
	return endpoint
}

func (person *Person) shop() {
	shopEndpoint := person.pickShop()

	// First check on the status of open orders.
	for orderId, _ := range person.openOrders {
		order, err := shopApi.GetOrder(shopEndpoint, orderId)
		person.ShopRequests++
		if person.error(err) {
			return
		}
		if !order.IsProcessing() {
			delete(person.openOrders, orderId)
		}
	}

	// Don't continue shopping if too many orders are open
	if person.OpenOrdersLimit > 0 && len(person.openOrders) >= int(person.OpenOrdersLimit) {
		person.SkippedShopping++
		services.L.LogLevelf(services.LevelNormal+1,
			"%v skipping shopping because %v orders are already open", person.Name, len(person.openOrders))
		return
	}

	items, err := shopApi.AllItems(shopEndpoint)
	person.ShopRequests++
	if person.error(err) {
		return
	}
	item_index := rand.Uint32() % uint32(len(items))
	item := items[item_index].Name
	services.L.Logf("%v ordering %v", person.Name, item)
	id, err := shopApi.PlaceOrder(shopEndpoint, person.Name, item, 1)
	person.ShopRequests++
	if person.error(err) {
		return
	}
	person.openOrders[id] = true
}

func (person *Person) sleep() {
	time.Sleep(person.sleepTime)
}
