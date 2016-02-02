package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/pborman/uuid"

	"github.com/antongulenko/http-isolation-proxy/services"
	"github.com/antongulenko/http-isolation-proxy/services/service_catalog/catalogApi"
	"github.com/antongulenko/http-isolation-proxy/services/service_shop/shopApi"
)

const (
	orders_key       = "orders/"
	fresh_orders_key = "fresh_orders/"
	user_orders_key  = "user_orders/"
	open_orders_key  = "open_orders"

	fresh_order_timeout_sec = 2
)

type Shop struct {
	redis           services.Redis
	redisLockValue  string
	catalogEndpoint string
	paymentEndpoint string
}

type Item catalogApi.Item

type Order struct {
	shopApi.Order
	services.StoredObject `json:"-"`

	Timestamp  string `json:"-"`
	ShipmentId string `json:"-"`
	PaymentId  string `json:"-"`

	id   string
	shop *Shop
}

func (order *Order) Key() string {
	return orders_key + order.id
}

func (order *Order) Client() services.Redis {
	return order.shop.redis
}

func (shop *Shop) AllItems() ([]*Item, error) {
	items, err := catalogApi.AllItems(shop.catalogEndpoint)
	if err != nil {
		return nil, err
	}
	result := make([]*Item, len(items))
	for i, item := range items {
		result[i] = (*Item)(item)
	}
	return result, nil
}

func (shop *Shop) MakeOrder(id string) *Order {
	order := &Order{
		shop: shop,
		id:   id,
	}
	order.StoredObject = services.StoredObject{order}
	return order
}

func (shop *Shop) NewOrder(username string, item string, qty uint64) error {
	order_time := time.Now()
	timestamp := strconv.FormatUint(uint64(order_time.Unix()), 10)
	if err := shop.noteFreshOrder(username, item, qty); err != nil {
		return err
	}
	order := shop.MakeOrder(uuid.New())
	order.User = username
	order.Item = item
	order.Quantity = qty
	order.Timestamp = timestamp
	order.Status = "processing"
	order.Time = order_time.String()

	// TODO would be good to try to release the fresh_order lock if the transaction fails
	return shop.redis.Transaction(func() error {
		err := shop.redis.Cmd("sadd", user_orders_key+username, order.id).Err()
		if err != nil {
			return err
		}
		err = shop.redis.Cmd("sadd", open_orders_key, order.id).Err()
		if err != nil {
			return err
		}
		return order.Save()
	})
}

func (shop *Shop) noteFreshOrder(username string, item string, qty uint64) error {
	hash := services.MakeHash(username, item, qty)
	resp := shop.redis.Cmd("set", fresh_orders_key+hash, shop.redisLockValue, "ex", fresh_order_timeout_sec, "nx")
	if err := resp.Err(); err != nil {
		return err
	}
	if str, _ := resp.Str(); resp.HasResult() && str == "OK" {
		return nil
	} else {
		// The same order has been submitted within fresh_order_timeout
		return fmt.Errorf("Order rejected: duplicate order suspected")
	}
}

func (shop *Shop) AllOrders(username string) ([]*Order, error) {
	all_orders, err := shop.redis.Cmd("smembers", user_orders_key+username).List()
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch all orders for user %v: %v", username, err)
	}
	result := make([]*Order, len(all_orders))
	for i, order_id := range all_orders {
		order := shop.MakeOrder(order_id)
		existed, err := order.LoadExisting()
		if err != nil {
			return nil, fmt.Errorf("Failed to fetch order %v for user %v: %v", order_id, username, err)
		} else if !existed {
			return nil, fmt.Errorf("Registered order did not exist: %v", order_id)
		}
		result[i] = order
	}
	return result, nil
}
