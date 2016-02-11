package main

import (
	"strconv"
	"sync"

	"github.com/antongulenko/http-isolation-proxy/services"
)

type Pool struct {
	people []*Person
	active int
	wg     sync.WaitGroup

	bankEndpoint  string
	shopEndpoints []string
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

func (pool *Pool) addPerson() {
	person := RandomPerson("User"+strconv.Itoa(len(pool.people)), pool.bankEndpoint, pool.shopEndpoints)
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
