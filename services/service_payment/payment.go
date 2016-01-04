package main

import (
	"fmt"
	"time"

	"github.com/antongulenko/http-isolation-proxy/services"
	"github.com/antongulenko/http-isolation-proxy/services/service_bank/bankApi"
	"github.com/antongulenko/http-isolation-proxy/services/service_payment/paymentApi"
)

const (
	lock_expiration = 10 * time.Second
	lock_prefix     = "lock/"
	payment_prefix  = "payment/"
)

var (
	PaymentFailed    = PaymentStatus{&PaymentStatusValue{paymentApi.PaymentFailed, 0, false}}
	PaymentCancelled = PaymentStatus{&PaymentStatusValue{paymentApi.PaymentCancelled, 0, false}}

	PaymentCreated   = PaymentStatus{&PaymentStatusValue{paymentApi.PaymentCreated, 1, false}}
	PaymentPending   = PaymentStatus{&PaymentStatusValue{paymentApi.PaymentPending, 2, true}}
	PaymentCommitted = PaymentStatus{&PaymentStatusValue{paymentApi.PaymentCommitted, 3, true}}
	PaymentProcessed = PaymentStatus{&PaymentStatusValue{paymentApi.PaymentProcessed, 4, false}}
)

type PaymentStatus struct {
	*PaymentStatusValue
}

type PaymentStatusValue struct {
	str        string
	order      int
	canAdvance bool
}

func (st PaymentStatus) String() string {
	if st.PaymentStatusValue == nil {
		return "<no status>"
	}
	return st.str
}

type Payments struct {
	bank           bankApi.Bank
	redis          services.Redis
	accountName    string
	redisLockValue string
}

type Payment struct {
	lock     services.RedisLock
	payments *Payments
	paymentApi.Payment
	services.StoredObject `json:"-"`

	TransactionId string `json:"-" redis:"tid"`
	id            string
	status        PaymentStatus
}

func (payment *Payment) Key() string {
	return payment_prefix + payment.id
}

func (payment *Payment) Client() services.Redis {
	return payment.payments.redis
}

func (payments *Payments) MakePayment(id string) *Payment {
	payment := &Payment{
		id:       id,
		payments: payments,
		lock: services.RedisLock{
			Client:     payments.redis,
			LockName:   lock_prefix + payment_prefix + id,
			LockValue:  payments.redisLockValue,
			Expiration: lock_expiration,
		},
	}
	payment.StoredObject = services.StoredObject{payment}
	return payment
}

func (payments *Payments) NewPayment(username string, value float64, timestamp string) (*Payment, bool, error) {
	// Create a reproducible ID based on input data (hence the timestamp)
	hash := services.MakeHash(username, value, timestamp)

	// Try to fetch an existing payment
	payment := payments.MakePayment(hash)
	existed, err := payment.LoadExisting(true, false)
	if err != nil {
		return nil, false, err
	}
	defer payment.unlock()

	// Create the payment, if it did not exist
	if !existed {
		payment.User = username
		payment.Value = value
		payment.TransactionId = ""
		payment.setStatus(PaymentCreated)
		if err = payment.Save(); err != nil {
			return nil, false, err
		}
	}

	// Now try to create the remote transaction
	if err = payment.advance(PaymentPending); err != nil {
		return nil, false, err
	} else {
		return payment, existed, nil
	}
}

func (payment *Payment) LoadExisting(lockPayment bool, unlock_missing_payment bool) (bool, error) {
	auto_unlock := true
	if lockPayment {
		if err := payment.lock.Lock(); err != nil {
			// This means somebody is already working on this payment
			return false, fmt.Errorf("Failed to lock payment: %v", err)
		}
	}
	defer func() {
		if lockPayment && auto_unlock {
			payment.unlock()
		}
	}()

	exists, err := payment.StoredObject.LoadExisting()
	if err != nil {
		return false, err
	} else if !exists {
		auto_unlock = unlock_missing_payment
		return false, nil
	}

	// After loading our data, check the state of the remote transaction
	if err := payment.inferState(); err != nil {
		return false, fmt.Errorf("Failed to infer payment state: %v", err)
	}

	auto_unlock = false
	return true, nil
}

func (payment *Payment) unlock() {
	if err := payment.lock.Unlock(); err != nil {
		services.L.Warnf("Error releasing redis lock for payment:", err)
	}
}

func (payment *Payment) setStatus(status PaymentStatus) {
	payment.status = status
	payment.Status = status.String()
	if status != PaymentFailed {
		payment.Error = ""
	}
}

func (payment *Payment) Commit() error {
	defer payment.unlock()
	return payment.advance(PaymentCommitted)
}

func (payment *Payment) Cancel() error {
	defer payment.unlock()

	switch payment.status {
	case PaymentCreated, PaymentCancelled:
		// Transaction has not been created yet, or already cancelled
		return nil
	case PaymentPending, PaymentCommitted:
		trans, err := payment.getTransaction()
		if err != nil {
			return err
		}
		return trans.Cancel()
	case PaymentProcessed:
		trans, err := payment.getTransaction()
		if err != nil {
			return err
		}
		return trans.Revert()
	default:
		return services.Conflictf("Cannot cancel a %v payment", payment.status)
	}
}

func (payment *Payment) inferState() error {
	if payment.TransactionId == "" {
		payment.setStatus(PaymentCreated)
		return nil
	}
	if trans, err := payment.getTransaction(); err != nil {
		return err
	} else {
		switch state := trans.State(); state {
		case bankApi.TransactionPending:
			payment.setStatus(PaymentPending)
		case bankApi.TransactionCommitted:
			payment.setStatus(PaymentCommitted)
		case bankApi.TransactionProcessed:
			payment.setStatus(PaymentProcessed)
		case bankApi.TransactionReverted, bankApi.TransactionCancelled, bankApi.TransactionReverting:
			payment.setStatus(PaymentCancelled)
		default:
			payment.setStatus(PaymentFailed)
			payment.Error = fmt.Sprintf("Transaction %s, error: %s", trans.State(), trans.Error())
		}
	}
	return nil
}

func (payment *Payment) advance(targetState PaymentStatus) error {
	if !targetState.canAdvance {
		return fmt.Errorf("Won't advance payment to %v", targetState)
	}
	for payment.status.order < targetState.order {
		previousStatus := payment.status
		var err error
		switch previousStatus {
		case PaymentCreated:
			err = payment.advanceToPending()
		case PaymentPending:
			err = payment.advanceToCommitted()
		default:
			return services.Conflictf("Cannot advance %v payment", previousStatus)
		}
		if err != nil {
			return fmt.Errorf("Failed to advance %v payment: %v", previousStatus, err)
		}
		if previousStatus.order == payment.status.order {
			// Avoid infinite loop
			return fmt.Errorf("Status of %v payment did not change", previousStatus)
		}
	}
	return nil
}

func (payment *Payment) advanceToPending() error {
	trans, err := payment.payments.bank.PendingTransfer(payment.User, payment.payments.accountName, payment.Value)
	if err != nil {
		// Should probably retry here
		return fmt.Errorf("Failed to create pending transaction: %v", err)
	}
	payment.TransactionId = trans.Id()
	payment.setStatus(PaymentPending)
	if err := payment.Save(); err != nil {
		return fmt.Errorf("Failed to store id of pending transaction: %v", err)
	}
	return nil
}

func (payment *Payment) advanceToCommitted() error {
	trans, err := payment.getTransaction()
	if err != nil {
		return fmt.Errorf("Failed to retrieve transaction for %v payment: %v", payment.status, err)
	}
	if err := trans.Commit(); err != nil {
		return fmt.Errorf("Error committing pending transaction for %v payment: %v", payment.status, err)
	}
	payment.setStatus(PaymentCommitted)
	return nil
}

func (payment *Payment) getTransaction() (bankApi.Transaction, error) {
	if payment.TransactionId == "" {
		return nil, fmt.Errorf("Payment (%s) has no transaction Id...", payment.status)
	}
	trans, err := payment.payments.bank.GetTransaction(payment.TransactionId)
	if err != nil {
		return nil, fmt.Errorf("Error getting transaction of %v payment: %v", payment.status, err)
	}
	return trans, nil
}
