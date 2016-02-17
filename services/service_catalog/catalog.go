package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/antongulenko/http-isolation-proxy/services"
	"github.com/antongulenko/http-isolation-proxy/services/service_catalog/catalogApi"
)

const (
	item_lock_retries = 5
	lock_expiration   = 3 * time.Second
	item_prefix       = "item/"
	lock_prefix       = "lock/"
	all_items_key     = "all_items"
)

type Catalog struct {
	redis          services.Redis
	redisLockValue string
}

type Item struct {
	services.StoredObject `json:"-"`
	catalogApi.Item

	lock      sync.Mutex
	redisLock services.RedisLock
	catalog   *Catalog
}

func (item *Item) Key() string {
	return item_prefix + item.Name
}

func (item *Item) Client() services.Redis {
	return item.catalog.redis
}

func (catalog *Catalog) MakeItem(name string, initialStock uint64, cost float64) *Item {
	item := &Item{
		catalog: catalog,
		Item: catalogApi.Item{
			Name:  name,
			Stock: initialStock,
			Cost:  cost,
		},
		redisLock: services.RedisLock{
			Client:     catalog.redis,
			LockName:   lock_prefix + item_prefix + name,
			LockValue:  catalog.redisLockValue,
			Expiration: lock_expiration,
			Retry:      item_lock_retries, // This is a frequent lock, do a few retries
		},
	}
	item.StoredObject = services.StoredObject{item}
	return item
}

func (catalog *Catalog) ensureItems(items []*Item) error {
	for _, item := range items {
		if exists, err := item.Exists(); exists || err != nil {
			return err
		}
		if err := item.Save(); err != nil {
			return fmt.Errorf("Error storing item %s: %v", item.Name, err)
		}
		if err := catalog.redis.Cmd("sadd", all_items_key, item.Name).Err(); err != nil {
			return fmt.Errorf("Error adding %s to list of all items: %v", item.Name, err)
		}
	}
	return nil
}

func (catalog *Catalog) GetAllItems() ([]*Item, error) {
	all_items, err := catalog.redis.Cmd("smembers", all_items_key).List()
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve list of items: %v", err)
	}
	var items []*Item
	for _, item_name := range all_items {
		if item, err := catalog.GetItem(item_name); err != nil {
			return nil, fmt.Errorf("Error fetching item %v: %v", item_name, err)
		} else if item == nil {
			// Item does not exist, even though it's in the all_items list. Ignore.
			continue
		} else {
			items = append(items, item)
		}
	}
	return items, nil
}

func (catalog *Catalog) GetItem(name string) (*Item, error) {
	item := catalog.MakeItem(name, 0, 0)
	if exists, err := item.Exists(); !exists || err != nil {
		return nil, err
	} else if err := item.Load(); err != nil {
		return nil, err
	} else {
		return item, nil
	}
}

func (catalog *Catalog) RefillItems(timeout time.Duration, refills map[string]uint64) {
	for {
		time.Sleep(timeout)
		services.L.Logf("Refilling items...")
		for itemName, refill := range refills {
			item, err := catalog.GetItem(itemName)
			if err != nil {
				services.L.Warnf("Error getting item %v for refill: %v", itemName, err)
				continue
			}
			catalog.refillItem(item, refill)
		}
	}
}

func (catalog *Catalog) refillItem(item *Item, refill uint64) {
	err := item.redisLock.Transaction(func() error {
		if item.Stock == 0 {
			item.Stock = refill
			item.Refills += refill
			if err := item.Save(); err == nil {
				services.L.Warnf("Refilling item %s to %v", item.Name, refill)
			} else {
				services.L.Warnf("Error saving item %v after refilling to %v: %v", item.Name, refill, err)
				return err
			}
		} else {
			services.L.Logf("Not refilling %s, stock is still %v", item.Name, item.Stock)
		}
		return nil
	})
	if err != nil {
		services.L.Warnf("Failed to refill item %v: %v", item.Name, err)
	}
}
