package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

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

	bank         bankApi.Bank
	shopEndpoint string

	monthlyPay float64
	Name       string
}

func (person *Person) String() string {
	return fmt.Sprintf("%v: sleeps %v, earns %v", person.Name, person.sleepTime, person.monthlyPay)
}

func RandomPerson(name string, bankEndpoint string, shopEndpoint string) *Person {
	sleepTime := (time.Duration(rand.Int63n(max_sleep-min_sleep) + min_sleep)) * time.Millisecond
	monthlyPay := rand.Float64()*max_pay + min_pay

	return &Person{
		Name:         name,
		sleepTime:    sleepTime,
		monthlyPay:   monthlyPay,
		bank:         bankApi.NewHttpBank(bankEndpoint),
		shopEndpoint: shopEndpoint,
	}
}

func (person *Person) Live(wg *sync.WaitGroup) {
	wg.Add(1)
	go person.doLive()
}

func (person *Person) doLive() {
	day := start_day
	for {
		day++
		if day%days_per_month == 0 {
			person.earn()
		}
		person.shop()
		person.sleep()
	}
}

func (person *Person) error(err error) bool {
	if err == nil {
		return false
	} else {
		log.Printf("%v: %v\n", person.Name, err)
		return true
	}
}

func (person *Person) earn() {
	_, err := person.bank.Deposit(person.Name, person.monthlyPay)
	log.Printf("%v earning %v\n", person.Name, person.monthlyPay)
	person.error(err)
}

func (person *Person) shop() {
	items, err := shopApi.AllItems(person.shopEndpoint)
	if person.error(err) {
		return
	}
	item_index := rand.Uint32() % uint32(len(items))
	item := items[item_index].Name
	log.Printf("%v ordering %v\n", person.Name, item)
	err = shopApi.PlaceOrder(person.shopEndpoint, person.Name, item, 1)
	person.error(err)
}

func (person *Person) sleep() {
	time.Sleep(person.sleepTime)
}
