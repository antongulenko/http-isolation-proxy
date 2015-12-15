package main

import (
	"sync"
	"time"

	"github.com/antongulenko/http-isolation-proxy/services/service_bank/bankApi"

	"code.google.com/p/go-uuid/uuid"
)

const (
	transaction_processing_timeout = 3 * time.Second
	transaction_queue_size         = 50
)

type AccountStore struct {
	accounts         map[string]*Account
	lock             sync.Mutex
	transactionQueue chan *Transaction
	transactions     map[string]*Transaction
}

type Account struct {
	bankApi.HttpAccount

	lock sync.Mutex
}

func (account *Account) String() string {
	if account == nil {
		return "nil"
	}
	return account.Username
}

func NewAccountStore() *AccountStore {
	return &AccountStore{
		accounts:         make(map[string]*Account),
		transactionQueue: make(chan *Transaction, transaction_queue_size),
		transactions:     make(map[string]*Transaction),
	}
}

func (store *AccountStore) NewTransaction(account *Account, value float64, targetAccount *Account) *Transaction {
	trans := &Transaction{
		Id:            uuid.New(),
		account:       account,
		value:         value,
		targetAccount: targetAccount,
		store:         store,
	}
	trans.setState(TransactionPending)
	store.transactions[trans.Id] = trans
	return trans
}

func (store AccountStore) GetAccount(username string) *Account {
	store.lock.Lock()
	defer store.lock.Unlock()
	if account, ok := store.accounts[username]; ok {
		return account
	} else {
		account := &Account{}
		account.Username = username
		store.accounts[username] = account
		return account
	}
}

func (store *AccountStore) HandleTransactions() {
	for {
		trans := <-store.transactionQueue
		store.handleTransaction(trans)
	}
}

func (store *AccountStore) handleTransaction(trans *Transaction) {
	time.Sleep(transaction_processing_timeout)
	trans.Process()
}
