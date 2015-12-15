package main

import (
	"flag"
	"fmt"
	"strconv"
	"sync"
)

func main() {
	num_users := flag.Uint("users", 10, "Number of simulated people")
	bank := flag.String("bank", "localhost:9001", "Bank endpoint")
	shop := flag.String("shop", "localhost:9004", "Shop endpoint")
	flag.Parse()

	var wg sync.WaitGroup
	for i := uint(0); i < *num_users; i++ {
		person := RandomPerson("User"+strconv.Itoa(int(i)), *bank, *shop)
		fmt.Println(person)
		person.Live(&wg)
	}
	wg.Wait()
}
