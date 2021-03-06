package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/antongulenko/http-isolation-proxy/services"
	"github.com/antongulenko/http-isolation-proxy/services/service_catalog/catalogApi"
	"github.com/antongulenko/http-isolation-proxy/services/service_payment/paymentApi"
)

const (
	order_lock_prefix           = "order_lock/"
	order_scan_interval         = 1 * time.Second
	order_processing_expiration = 10 * time.Second
)

func (shop *Shop) LoopScanOrders(ids chan<- string) {
	for {
		start := time.Now()
		shop.scanOrders(ids)
		if delta := time.Now().Sub(start); delta < order_scan_interval {
			// Some delay to avoid constant load on Redis
			time.Sleep(order_scan_interval - delta)
		}
	}
}

func (shop *Shop) scanOrders(ids chan<- string) (result uint) {
	open_orders, err := shop.redis.Cmd("smembers", open_orders_key).List()
	if err != nil {
		services.L.Logf("Error retrieving list of open orders: %v", err)
		return
	}
	for _, order_id := range open_orders {
		ids <- order_id
		result++
	}
	return
}

func (shop *Shop) LoopProcessOrders(ids <-chan string) {
	for order_id := range ids {
		shop.processOrder(order_id)
	}
}

func (shop *Shop) processOrder(order_id string) {
	lock := services.RedisLock{
		Client:     shop.redis,
		LockName:   order_lock_prefix + order_id,
		LockValue:  shop.redisLockValue,
		Expiration: order_processing_expiration,
	}
	order := shop.MakeOrder(order_id)

	if err := lock.TryLock(); err != nil {
		// Somebody processing this order already
		return
	}
	defer func() {
		if err := lock.Unlock(); err != nil {
			services.L.Logf("Error unlocking order %v after processing: %v", order_id, err)
		}
	}()

	// Loading once is enough: all further calls are completely idempontent
	// The locking is purely for performance, to avoid unnecessary HTTP calls from multiple shop instances
	existed, err := order.LoadExisting()
	if err != nil {
		services.L.Logf("Error retrieving order: %v", err)
	} else if !existed {
		services.L.Warnf("Locked order %v does not exist", order_id)
	}

	item, err := catalogApi.GetItem(shop.catalogEndpoint, order.Item)
	if err != nil {
		services.L.Warnf("Failed to retrieve item '%s' for order processing: %v", order.Item, err)
	}
	shop.doProcessOrder(order, item)
}

func (shop *Shop) doProcessOrder(order *Order, item *catalogApi.Item) {
	if order.assertShipment(item) &&
		order.assertPayment(item) &&
		order.commitShipment() &&
		order.commitPayment() &&
		order.deliverShipment() {
		order.Finalize()
	}
}

func (order *Order) assertShipment(item *catalogApi.Item) bool {
	if order.ShipmentId == "" {
		id, err := catalogApi.ShipItem(order.shop.catalogEndpoint, order.Item, order.User, order.Quantity, order.Timestamp)
		if err != nil {
			services.L.Logf("Failed to create item shipment: %v", err)
			return false
		}
		order.ShipmentId = id
		if err := order.Save(); err != nil {
			services.L.Logf("Failed to store shipment ID for order: %v", err)
			return false
		}
	}
	return true
}

func (order *Order) assertPayment(item *catalogApi.Item) bool {
	if order.PaymentId == "" {
		totalCost := float64(order.Quantity) * item.Cost
		id, err := paymentApi.CreatePayment(order.shop.paymentEndpoint, order.User, totalCost, order.Timestamp)
		if err != nil {
			services.L.Logf("Failed to create payment: %v", err)
			return false
		}
		order.PaymentId = id
		if err := order.Save(); err != nil {
			services.L.Logf("Failed to store payment ID for order: %v", err)
			return false
		}
	}
	return true
}

func (order *Order) shipmentStatus() catalogApi.ShipmentStatus {
	shipment, err := catalogApi.GetShipment(order.shop.catalogEndpoint, order.ShipmentId)
	if order.checkError(err) {
		return ""
	}
	return shipment.Status
}

func (order *Order) checkShipmentStatus(expectedStatus catalogApi.ShipmentStatus) bool {
	if shipmentStatus := order.shipmentStatus(); shipmentStatus == "" {
		return false
	} else if shipmentStatus != expectedStatus {
		services.L.Logf("Shipment %v did not change to status %v (instead %v)", order.ShipmentId, expectedStatus, shipmentStatus)
		return false
	}
	return true
}

func (order *Order) commitShipment() bool {
	shipmentStatus := order.shipmentStatus()
	if shipmentStatus == "" {
		return false
	}
	switch shipmentStatus {
	case catalogApi.ShipmentCreated:
		err := catalogApi.CommitShipment(order.shop.catalogEndpoint, order.ShipmentId)
		if order.checkError(err) {
			return false
		}
		return order.checkShipmentStatus(catalogApi.ShipmentCommitted)
	case catalogApi.ShipmentCommitted, catalogApi.ShipmentDelivered:
		return true
	default:
		order.Cancel(fmt.Errorf("Shipment has status %v", shipmentStatus))
		return false
	}
}

func (order *Order) commitPayment() bool {
	payment, err := paymentApi.FetchPayment(order.shop.paymentEndpoint, order.PaymentId)
	if order.checkError(err) {
		return false
	}
	switch payment.Status {
	case paymentApi.PaymentCreated, paymentApi.PaymentPending:
		err := paymentApi.CommitPayment(order.shop.paymentEndpoint, order.PaymentId)
		_ = order.checkError(err)
		// TODO maybe check that state changed to committed?
		return false
	case paymentApi.PaymentCommitted:
		// Waiting for payment to be processed
		return false
	case paymentApi.PaymentProcessed:
		return true
	default:
		order.Cancel(fmt.Errorf("Payment has status %v", payment.Status))
		return false
	}
}

func (order *Order) deliverShipment() bool {
	err := catalogApi.DeliverShipment(order.shop.catalogEndpoint, order.ShipmentId)
	if order.checkError(err) {
		return false
	}
	return order.checkShipmentStatus(catalogApi.ShipmentDelivered)
}

func isConflictError(err error) bool {
	if httpErr, ok := err.(*services.HttpStatusError); ok {
		return httpErr.Code == http.StatusConflict
	}
	return false
}

// Return true, if we should retry later.
// In case of a CONFLICT 408 status, cancel the order.
func (order *Order) checkError(err error) bool {
	if err == nil {
		return false
	}
	if isConflictError(err) {
		order.Cancel(err)
		return true
	}
	services.L.Logf("Error processing order %v: %v", order.id, err)
	return true
}

func (order *Order) Finalize() {
	services.L.Logf("Finalizing order %v", order.id)
	logStr := "Order processed successfully"
	if err := order.doCancel(true, logStr); err != nil {
		services.L.Logf("Finalizing order %v failed: %v", order.id, err)
	}
}

func (order *Order) Cancel(cause error) {
	services.L.Logf("Cancelling order %v because of: %v", order.id, cause)
	logStr := fmt.Sprintf("Cancelling because of: %v", cause)
	if err := order.doCancel(false, logStr); err != nil {
		services.L.Logf("Cancelling order %v failed: %v", order.id, err)
	}
}

// On non-conflict errors, retry cancelling the order later
func (order *Order) doCancel(success bool, log string) error {
	cancelLog := log

	// Try to cancel the shipment, if necessary
	shipment, err := catalogApi.GetShipment(order.shop.catalogEndpoint, order.ShipmentId)
	if err != nil {
		return err
	}
	if !success && shipment.Status != catalogApi.ShipmentCancelled {
		err := catalogApi.CancelShipment(order.shop.catalogEndpoint, order.ShipmentId)
		if err != nil {
			if isConflictError(err) {
				cancelLog += fmt.Sprintf("\nError cancelling shipment: %v", err)
			} else {
				return err
			}
		} else {
			cancelLog += "\nShipment cancelled"
		}
	} else {
		cancelLog += "\nShipment " + string(shipment.Status)
	}

	// Try to cancel the payment, if necessary
	payment, err := paymentApi.FetchPayment(order.shop.paymentEndpoint, order.PaymentId)
	if err != nil {
		return err
	}
	if !success && payment.Status != paymentApi.PaymentFailed {
		err := paymentApi.CancelPayment(order.shop.paymentEndpoint, order.PaymentId)
		if err != nil {
			if isConflictError(err) {
				cancelLog += fmt.Sprintf("\nError cancelling payment: %v", err)
			} else {
				return err
			}
		} else {
			cancelLog += "\nPayment cancelled"
		}
	}
	cancelLog += "\nPayment was " + payment.Status
	if payment.Error != "" {
		cancelLog += ", error: " + payment.Error
	}

	// Remove the order from the list of open orders & store cancel log
	return order.shop.redis.Transaction(func(redis services.Redis) error {
		order.Status = cancelLog
		err := redis.Cmd("srem", open_orders_key, order.id).Err()
		if err != nil {
			return err
		}
		return order.SaveIn(redis)
	})
}
