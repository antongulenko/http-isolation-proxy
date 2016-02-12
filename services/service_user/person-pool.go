package main

import (
	"strconv"
	"sync"
	"time"

	"github.com/antongulenko/http-isolation-proxy/services"
)

type Pool struct {
	people []*Person
	active int
	wg     sync.WaitGroup

	bankEndpoint  string
	shopEndpoints []string
	startTime     time.Time

	OrdersPerPerson uint
}

func NewPool(bankEndpoint string, shopEndpoints []string) *Pool {
	return &Pool{
		bankEndpoint:  bankEndpoint,
		shopEndpoints: shopEndpoints,
	}
}

func (pool *Pool) Terminate() {
	services.L.Warnf("Terminating...")
	for _, person := range pool.people {
		person.Terminate()
	}
}

func (pool *Pool) Wait() {
	pool.wg.Wait()
}

func (pool *Pool) Start(num int) {
	pool.startTime = time.Now()
	for i := 0; i < num; i++ {
		pool.StartOne()
	}
}

func (pool *Pool) StartOne() {
	if pool.active >= len(pool.people) {
		pool.addPerson()
	}
	pool.activatePerson()
}

func (pool *Pool) Pause(num int) {
	for i := 0; i < num; i++ {
		pool.PauseOne()
	}
}

func (pool *Pool) PauseOne() {
	if pool.active > 0 {
		person := pool.people[pool.active-1]
		person.Pause()
		pool.active--
		services.L.Warnf("Paused, now active: %v. Person: %v", pool.active, person)
	}
}

func (pool *Pool) PrintStats() {
	t := time.Now().Sub(pool.startTime)
	secs := float64(t.Seconds())
	var bankRequests uint64
	var shopRequests uint64
	var totalErrors uint64
	var totalSkippedShopping uint64
	for _, person := range pool.people {
		bankRequests += person.BankRequests
		shopRequests += person.ShopRequests
		totalErrors += person.TotalErrors
		totalSkippedShopping += person.SkippedShopping
	}
	services.L.Warnf(
		"Pool statistics:\nRuntime: %v\nSkipped shopping: %v\n"+
			"Shop Requests: %v\nShop Requests/second: %v\nBank Requests: %v\nBank Requests/second: %v\nTotal Errors: %v",
		t, totalSkippedShopping, shopRequests, float64(shopRequests)/secs, bankRequests, float64(bankRequests)/secs, totalErrors)
}

func (pool *Pool) addPerson() {
	person := RandomPerson("User"+strconv.Itoa(len(pool.people)), pool.bankEndpoint, pool.shopEndpoints)
	person.OpenOrdersLimit = pool.OrdersPerPerson
	pool.people = append(pool.people, person)
	person.Live(&pool.wg)
}

func (pool *Pool) activatePerson() {
	if len(pool.people) > pool.active {
		person := pool.people[pool.active]
		person.Start()
		pool.active++
		services.L.Warnf("Activated, now active: %v. Person: %v", pool.active, person)
	}
}
