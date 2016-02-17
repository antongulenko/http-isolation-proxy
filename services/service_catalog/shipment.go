package main

import (
	"fmt"

	"github.com/antongulenko/http-isolation-proxy/services"
	"github.com/antongulenko/http-isolation-proxy/services/service_catalog/catalogApi"
)

const (
	shipment_prefix = "shipment/"
)

type Shipment struct {
	id      string
	catalog *Catalog
	lock    services.RedisLock

	services.StoredObject `json:"-"`
	catalogApi.Shipment
}

func (shipment *Shipment) Key() string {
	return shipment_prefix + shipment.id
}

func (shipment *Shipment) Client() services.Redis {
	return shipment.catalog.redis
}

func (catalog *Catalog) MakeShipment(id string) *Shipment {
	shipment := &Shipment{
		id:      id,
		catalog: catalog,
		lock: services.RedisLock{
			Client:     catalog.redis,
			LockName:   lock_prefix + shipment_prefix + id,
			LockValue:  catalog.redisLockValue,
			Expiration: lock_expiration,
		},
	}
	shipment.StoredObject = services.StoredObject{shipment}
	return shipment
}

func (item *Item) Ship(user string, qty uint64, timestamp string) (*Shipment, bool, error) {
	// Create a reproducible ID based on input data (hence the timestamp)
	hash := services.MakeHash(user, item.Name, qty, timestamp)
	shipment := item.catalog.MakeShipment(hash)

	if err := shipment.lock.Lock(); err != nil {
		return nil, false, fmt.Errorf("Failed to lock shipment: %v", err)
	}
	defer shipment.unlock()

	existed, err := shipment.LoadExisting()
	if err != nil {
		return nil, false, err
	}

	if !existed {
		shipment.User = user
		shipment.Item = item.Name
		shipment.Quantity = qty
		shipment.Status = catalogApi.ShipmentCreated
		if err = shipment.Save(); err != nil {
			return nil, false, err
		}
	}
	return shipment, existed, nil
}

func (shipment *Shipment) unlock() {
	if err := shipment.lock.Unlock(); err != nil {
		services.L.Warnf("Error releasing redis lock for shipment:", err)
	}
}

func (shipment *Shipment) Commit() error {
	if err := shipment.lock.Lock(); err != nil {
		return fmt.Errorf("Failed to lock shipment: %v", err)
	}
	defer shipment.unlock()

	switch shipment.Status {
	case catalogApi.ShipmentCreated:
		return shipment.doCommit()
	case catalogApi.ShipmentCommitted, catalogApi.ShipmentDelivered:
		return nil
	default:
		return services.Conflictf("Cannot commit %v shipment", shipment.Status)
	}
}

func (shipment *Shipment) Cancel() error {
	if err := shipment.lock.Lock(); err != nil {
		return fmt.Errorf("Failed to lock shipment: %v", err)
	}
	defer shipment.unlock()

	switch shipment.Status {
	case catalogApi.ShipmentCreated:
		return shipment.doQuickCancel()
	case catalogApi.ShipmentCommitted:
		return shipment.doCancel()
	case catalogApi.ShipmentCancelled:
		return nil
	default:
		return services.Conflictf("Cannot cancel %v shipment", shipment.Status)
	}
}

func (shipment *Shipment) Deliver() error {
	if err := shipment.lock.Lock(); err != nil {
		return fmt.Errorf("Failed to lock shipment: %v", err)
	}
	defer shipment.unlock()

	switch shipment.Status {
	case catalogApi.ShipmentCreated:
		if err := shipment.doCommit(); err != nil {
			return err
		}
		if shipment.Status != catalogApi.ShipmentCommitted {
			return fmt.Errorf("Shipment did not change to %v status", catalogApi.ShipmentCommitted)
		}
		return shipment.doDeliver()
	case catalogApi.ShipmentCommitted:
		return shipment.doDeliver()
	case catalogApi.ShipmentDelivered:
		return nil
	default:
		return services.Conflictf("Cannot deliver %v shipment", shipment.Status)
	}
}

func (shipment *Shipment) modifyItem(description string, modify func(item *Item) error) error {
	item := shipment.catalog.MakeItem(shipment.Item, 0, 0)
	err := item.redisLock.Transaction(func() error {
		if err := item.Load(); err != nil {
			return err
		}
		if err := modify(item); err != nil {
			return err
		}
		if err := item.Save(); err != nil {
			return err
		}
		if err := shipment.Save(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("Error updating item for %v shipment: %v", description, err)
	}
	return nil
}

func (shipment *Shipment) doCommit() error {
	return shipment.modifyItem("committing",
		func(item *Item) error {
			if item.Stock < shipment.Quantity {
				return services.Conflictf("Cannot commit %v %v: Only %v in stock", shipment.Quantity, item.Name, item.Stock)
			}
			item.Stock -= shipment.Quantity
			item.Reserved += shipment.Quantity
			shipment.Status = catalogApi.ShipmentCommitted
			return nil
		})
}

func (shipment *Shipment) doQuickCancel() error {
	shipment.Status = catalogApi.ShipmentCancelled
	return shipment.Save()
}

func (shipment *Shipment) doCancel() error {
	return shipment.modifyItem("cancelling",
		func(item *Item) error {
			if item.Reserved < shipment.Quantity {
				services.L.Warnf("Inconsistency warning: Cancelling %v %v, but only %v were reserved\n", shipment.Quantity, item.Name, item.Reserved)
				item.Reserved = 0
			} else {
				item.Reserved -= shipment.Quantity
			}
			item.Stock += shipment.Quantity
			shipment.Status = catalogApi.ShipmentCancelled
			return nil
		})
}

func (shipment *Shipment) doDeliver() error {
	return shipment.modifyItem("delivering",
		func(item *Item) error {
			if item.Reserved < shipment.Quantity {
				return services.Conflictf("Cannot deliver %v %v: Only %v reserved", shipment.Quantity, item.Name, item.Reserved)
			}
			item.Reserved -= shipment.Quantity
			item.Shipped += shipment.Quantity
			shipment.Status = catalogApi.ShipmentDelivered
			return nil
		})
}
