package main

import "sync"

type OrderPool struct {
	ParallelOrders uint
	Bank           string
	Shops          []string
	User           string

	running bool
	orders  []*OrderRoutine
	wg      sync.WaitGroup
}

func (pool *OrderPool) Start() {
	pool.running = true
	pool.orders = make([]*OrderRoutine, pool.ParallelOrders)
	for i := uint(0); i < pool.ParallelOrders; i++ {
		order := &OrderRoutine{
			pool: pool,
		}
		pool.orders[i] = order
		order.Start()
	}
}

func (pool *OrderPool) Wait() {
	pool.wg.Wait()
}

func (pool *OrderPool) Terminate() {
	pool.running = false
}

func (pool *OrderPool) PrintStats() {
	// TODO summarize order statistics
}

func (pool *OrderPool) LoopPrintStats() {

}

type OrderRoutine struct {
	pool *OrderPool
}

func (order *OrderRoutine) Start() {
	wg := &order.pool.wg
	wg.Add(1)
	go func() {
		defer wg.Done()
		order.work()
	}()
}

func (order *OrderRoutine) work() {
	for order.pool.running {
		// TODO
		// place order, record statistics
	}
}
