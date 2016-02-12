package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"strings"

	"github.com/antongulenko/http-isolation-proxy/services/service_bank/bankApi"
	"github.com/antongulenko/http-isolation-proxy/services/service_shop/shopApi"
)

func check(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

var (
	initialItems = map[string]uint64{
		"DVD":       5000,
		"Toaster":   1000,
		"Laptop":    500,
		"TV":        100,
		"Spaceship": 0,
	}
	num_users = 1000
)

func roundInt(val float64) int {
	return int(val + math.Copysign(0.5, val))
}

func round(val float64) float64 {
	const places = 10000
	return float64(roundInt(val*places)) / places
}

func main() {
	bankEndpoint := flag.String("bank", "localhost:9001", "Bank endpoint")
	var shopEndpoint string
	flag.StringVar(&shopEndpoint, "shop", "localhost:9004", "Shop endpoint")
	flag.Parse()
	bank := bankApi.NewHttpBank(*bankEndpoint)
	inconsistent := false

	allItems, err := shopApi.AllItems(shopEndpoint)

	itemMap := make(map[string]*shopApi.Item)
	var totalEarned float64
	var totalShipped uint64
	for _, item := range allItems {
		itemMap[item.Name] = item
		total := item.Reserved + item.Shipped + item.Stock
		initial := initialItems[item.Name]
		expectedTotal := initial + item.Refills
		if total != expectedTotal {
			fmt.Printf("%v inconsistent: %v + %v + %v = %v, expected %v + %v = %v\n", item.Name, item.Reserved, item.Shipped, item.Stock, total, initial, item.Refills, expectedTotal)
			inconsistent = true
		}
		if item.Reserved != 0 {
			fmt.Printf("%v still has %v reserved items\n", item.Name, item.Reserved)
			inconsistent = true
		}
		totalShipped += item.Shipped
		totalEarned += float64(item.Shipped) * item.Cost
	}

	var cancelledOrders uint64
	var processedOrders uint64
	var totalEarnedOrders float64
	var totalShippedOrders uint64
	var totalProcessingOrders uint64
	for i := 0; i < num_users; i++ {
		user := fmt.Sprintf("User%v", i)

		orders, err := shopApi.AllOrders(shopEndpoint, user)
		check(err)
		for _, order := range orders {
			if strings.HasPrefix(order.Status, "Order processed successfully") {
				item := itemMap[order.Item]
				totalShippedOrders += order.Quantity
				totalEarnedOrders += float64(order.Quantity) * item.Cost
				processedOrders++
			} else if strings.HasPrefix(order.Status, "Cancelling because of:") {
				cancelledOrders++
			} else if order.Status == "processing" {
				totalProcessingOrders++
			} else {
				fmt.Println("Unknown order status:", order.Status)
				inconsistent = true
				break
			}
		}
		if inconsistent {
			break
		}
	}
	fmt.Println("Orders processed:", processedOrders, "orders cancelled:", cancelledOrders)
	if totalProcessingOrders > 0 {
		log.Println("UNFINISHED: There are still %v orders to process", totalProcessingOrders)
		inconsistent = true
	}
	if totalShipped != totalShippedOrders {
		fmt.Printf("Inconsistent: shipped %v, shipped orders %v\n", totalShipped, totalShippedOrders)
		inconsistent = true
	}

	balance, err := bank.Balance("store")
	check(err)

	balance = round(balance)
	totalEarnedOrders = round(totalEarnedOrders)
	totalEarned = round(totalEarned)

	if balance != totalEarned {
		fmt.Printf("Inconsistent earnings bank vs. items. Expected %v, have %v\n", totalEarned, balance)
		inconsistent = true
		if balance > totalEarned {
			fmt.Println(balance-totalEarned, "too much")
		} else {
			fmt.Println("Missing", totalEarned-balance)
		}
	}
	if balance != totalEarnedOrders {
		fmt.Printf("Inconsistent earnings bank vs. orders. Expected %v, have %v\n", totalEarnedOrders, balance)
		if balance > totalEarnedOrders {
			fmt.Println(balance-totalEarnedOrders, "too much")
		} else {
			fmt.Println("Missing", totalEarnedOrders-balance)
			inconsistent = true
		}
	}
	fmt.Printf("Total shipped items %v, total earnings %v\n", totalShipped, totalEarned)

	if inconsistent {
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}
