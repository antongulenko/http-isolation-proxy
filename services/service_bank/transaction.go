package main

import (
	"fmt"
	"sync"

	"github.com/antongulenko/http-isolation-proxy/services"
)

const (
	TransactionPending = TransactionState(iota)
	TransactionCommitted
	TransactionProcessed
	TransactionCancelled
	TransactionFailed
	TransactionReverting
	TransactionReverted
	TransactionRevertFailed
)

type TransactionState int

func (state TransactionState) String() string {
	switch state {
	case TransactionPending:
		return "pending"
	case TransactionCommitted:
		return "committed"
	case TransactionReverting:
		return "reverting"
	case TransactionProcessed:
		return "processed"
	case TransactionCancelled:
		return "cancelled"
	case TransactionFailed:
		return "failed"
	case TransactionReverted:
		return "reverted"
	case TransactionRevertFailed:
		return "revert-failed"
	default:
		return "unknown"
	}
}

type Transaction struct {
	Id    string `json:"id"`
	State string `json:"state"`
	Error string `json:"error"`

	account       *Account
	value         float64
	targetAccount *Account
	lock          sync.Mutex
	store         *AccountStore
	state         TransactionState
}

func (trans *Transaction) String() string {
	var target string
	if trans.targetAccount != nil {
		target = " -> " + trans.targetAccount.String()
	}
	return fmt.Sprintf("Transaction %v %v (%v, %v%s)", trans.Id, trans.State, trans.value, trans.account, target)
}

func (trans *Transaction) setState(state TransactionState) {
	trans.state = state
	trans.State = state.String()
	services.L.Tracef("%v\n", trans)
}

func (trans *Transaction) setError(err string) {
	trans.Error = err
	if trans.state == TransactionReverting {
		trans.state = TransactionRevertFailed
	} else {
		trans.state = TransactionFailed
	}
}

func (trans *Transaction) Cancel() error {
	trans.lock.Lock()
	defer trans.lock.Unlock()
	if trans.state != TransactionPending && trans.state != TransactionCommitted {
		return fmt.Errorf("Cannot cancel transaction in state %s", trans.state)
	}
	trans.setState(TransactionCancelled)
	return nil
}

func (trans *Transaction) Commit() error {
	trans.lock.Lock()
	defer trans.lock.Unlock()
	if trans.state != TransactionPending {
		return fmt.Errorf("Cannot commit transaction in state %s", trans.state)
	}
	trans.setState(TransactionCommitted)
	trans.store.transactionQueue <- trans
	return nil
}

func (trans *Transaction) Revert() error {
	trans.lock.Lock()
	defer trans.lock.Unlock()
	if trans.state != TransactionProcessed {
		return fmt.Errorf("Cannot revert transaction in state %s", trans.state)
	}
	if trans.targetAccount == nil {
		return fmt.Errorf("Cannot revert a deposit-transaction")
	}
	newTarget := trans.account
	trans.account = trans.targetAccount
	trans.targetAccount = newTarget
	trans.setState(TransactionReverting)
	trans.store.transactionQueue <- trans
	return nil
}

func (trans *Transaction) Process() {
	trans.lock.Lock()
	defer trans.lock.Unlock()
	if trans.state != TransactionCommitted && trans.state != TransactionReverting {
		return
	}

	trans.account.lock.Lock()
	defer trans.account.lock.Unlock()
	if trans.account.Balance < trans.value {
		trans.Error = fmt.Sprintf("Insufficient funds on account %v: %v (but %v required)", trans.account.Username, trans.account.Balance, trans.value)
		if trans.state == TransactionCommitted {
			trans.setState(TransactionFailed)
		} else {
			trans.setState(TransactionRevertFailed)
		}
		return
	}

	if trans.targetAccount != nil {
		trans.targetAccount.lock.Lock()
		defer trans.targetAccount.lock.Unlock()
		trans.targetAccount.Balance += trans.value
		trans.targetAccount.NumTransactions++
	}
	trans.account.Balance -= trans.value
	trans.account.NumTransactions++
	if trans.state == TransactionCommitted {
		trans.setState(TransactionProcessed)
	} else {
		trans.setState(TransactionReverted)
	}
}
