package bankApi

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/antongulenko/http-isolation-proxy/services"
)

const (
	TransactionPending      = "pending"
	TransactionCommitted    = "committed"
	TransactionReverting    = "reverting"
	TransactionProcessed    = "processed"
	TransactionCancelled    = "cancelled"
	TransactionFailed       = "failed"
	TransactionReverted     = "reverted"
	TransactionRefertFailed = "revert-failed"
)

type HttpAccount struct {
	Username        string  `json:"name"`
	Balance         float64 `json:"balance"`
	NumTransactions uint    `json:"transactions"`
}

type Bank interface {
	Balance(account string) (float64, error)
	PendingDeposit(account string, value float64) (Transaction, error)
	Deposit(account string, value float64) (Transaction, error)
	PendingTransfer(from, to string, value float64) (Transaction, error)
	Transfer(from, to string, value float64) (Transaction, error)
	GetTransaction(id string) (Transaction, error)
}

type Transaction interface {
	// Fetch remote information
	Update() error

	// Change status
	Commit() error
	Cancel() error
	Revert() error

	// Query information
	Id() string
	State() string
	Error() string
}

type HttpBank struct {
	endpoint string
}

type HttpTransaction struct {
	id    string
	state string
	error string
	bank  *HttpBank
}

func NewHttpBank(endpoint string) Bank {
	return &HttpBank{
		endpoint: endpoint,
	}
}

func (bank *HttpBank) Balance(account string) (float64, error) {
	the_url := "http://" + bank.endpoint + "/account/" + account
	var result HttpAccount
	err := services.Http_get_json(the_url, &result)
	if err != nil {
		return 0, err
	}
	return result.Balance, nil
}

func (bank *HttpBank) checkTransactionResponse(data string, err error, url string) (Transaction, error) {
	if err != nil {
		return nil, err
	}
	return &HttpTransaction{
		id:   (string)(data),
		bank: bank,
	}, nil
}

func (bank *HttpBank) PendingTransfer(from, to string, value float64) (Transaction, error) {
	return bank.transfer(from, to, value, false)
}

func (bank *HttpBank) Transfer(from, to string, value float64) (Transaction, error) {
	return bank.transfer(from, to, value, true)
}

func (bank *HttpBank) transfer(from, to string, value float64, auto_commit bool) (Transaction, error) {
	the_url := "http://" + bank.endpoint + "/account/" + from + "/transfer"
	resp, err := services.Http_post_string(the_url,
		url.Values{
			"target": []string{to},
			"value":  []string{fmt.Sprintf("%v", value)},
			"commit": []string{strconv.FormatBool(auto_commit)},
		})
	return bank.checkTransactionResponse(resp, err, the_url)
}

func (bank *HttpBank) Deposit(account string, value float64) (Transaction, error) {
	return bank.deposit(account, value, true)
}

func (bank *HttpBank) PendingDeposit(account string, value float64) (Transaction, error) {
	return bank.deposit(account, value, false)
}

func (bank *HttpBank) deposit(account string, value float64, auto_commit bool) (Transaction, error) {
	the_url := "http://" + bank.endpoint + "/account/" + account + "/deposit"
	resp, err := services.Http_post_string(the_url,
		url.Values{
			"value":  []string{fmt.Sprintf("%v", value)},
			"commit": []string{strconv.FormatBool(auto_commit)},
		})
	return bank.checkTransactionResponse(resp, err, the_url)
}

func (bank *HttpBank) GetTransaction(id string) (Transaction, error) {
	tran := &HttpTransaction{
		id:   id,
		bank: bank,
	}
	if err := tran.Update(); err != nil {
		return nil, err
	} else {
		return tran, nil
	}
}

func (trans *HttpTransaction) Update() error {
	the_url := "http://" + trans.bank.endpoint + "/transaction/" + trans.id
	data, err := services.Http_get_json_map(the_url, "state", "error")
	if err != nil {
		return err
	}
	var ok bool
	if trans.state, ok = data["state"].(string); !ok {
		return fmt.Errorf("Illegal JSON response, 'state' key is not string: %v", data)
	}
	if trans.error, ok = data["error"].(string); !ok {
		return fmt.Errorf("Illegal JSON response, 'error' key is not string: %v", data)
	}
	return nil
}

func (trans *HttpTransaction) performAction(action string) error {
	the_url := "http://" + trans.bank.endpoint + "/transaction/" + trans.id + "/" + action
	err := services.Http_simple_post(the_url)
	if err != nil {
		return fmt.Errorf("Failed to %s transaction: %v", action, err)
	}
	return nil
}

func (trans *HttpTransaction) Commit() error {
	return trans.performAction("commit")
}

func (trans *HttpTransaction) Cancel() error {
	return trans.performAction("cancel")
}

func (trans *HttpTransaction) Revert() error {
	return trans.performAction("revert")
}

func (trans *HttpTransaction) State() string {
	return trans.state
}

func (trans *HttpTransaction) Error() string {
	return trans.error
}

func (tran *HttpTransaction) Id() string {
	return tran.id
}
