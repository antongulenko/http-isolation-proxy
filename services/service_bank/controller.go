package main

import (
	"net/http"
	"strconv"

	"github.com/antongulenko/http-isolation-proxy/services"
	"github.com/gorilla/mux"
)

func (store *AccountStore) show_stats(w http.ResponseWriter, r *http.Request) {
	stats := store.Stats()
	services.Http_respond_json(w, r, stats)
}

func (store *AccountStore) get_account(w http.ResponseWriter, r *http.Request) *Account {
	username := mux.Vars(r)["id"]
	if account := store.GetAccount(username); account == nil {
		services.Http_respond_error(w, r, "Account not found: "+username, http.StatusNotFound)
		return nil
	} else {
		return account
	}
}

func (store *AccountStore) optionally_commit_transaction(trans *Transaction, w http.ResponseWriter, r *http.Request) bool {
	commit := false
	if commitStr := r.FormValue("commit"); commitStr == "true" {
		commit = true
	}
	if commit {
		err := trans.Commit()
		if err != nil {
			services.Http_respond_error(w, r, "Unexpected failure to commit transaction: "+err.Error(), http.StatusInternalServerError)
			return false
		}
	}
	return true
}

func (store *AccountStore) get_value_query(w http.ResponseWriter, r *http.Request) (float64, bool) {
	value, err := strconv.ParseFloat(r.FormValue("value"), 64)
	if err != nil {
		services.Http_respond_error(w, r, "Failed to parse 'value' parameter to float: "+err.Error(), http.StatusBadRequest)
		return 0, false
	}
	return value, true
}

func (store *AccountStore) handle_deposit(w http.ResponseWriter, r *http.Request) {
	account := store.get_account(w, r)
	value, ok := store.get_value_query(w, r)
	if !ok {
		return
	}

	trans := store.NewTransaction(account, -value, nil)
	if store.optionally_commit_transaction(trans, w, r) {
		services.Http_respond(w, r, ([]byte)(trans.Id), http.StatusOK)
	}
}

func (store *AccountStore) handle_transfer(w http.ResponseWriter, r *http.Request) {
	account := store.get_account(w, r)
	value, ok := store.get_value_query(w, r)
	if !ok {
		return
	}

	target := r.FormValue("target")
	if target == "" {
		services.Http_respond_error(w, r, "Need 'target' string parameter", http.StatusBadRequest)
		return
	}
	targetAccount := store.GetAccount(target)

	trans := store.NewTransaction(account, value, targetAccount)
	if store.optionally_commit_transaction(trans, w, r) {
		services.Http_respond(w, r, ([]byte)(trans.Id), http.StatusOK)
	}
}

func (store *AccountStore) show_account(w http.ResponseWriter, r *http.Request) {
	if account := store.get_account(w, r); account != nil {
		services.Http_respond_json(w, r, account)
	}
}

func (store *AccountStore) get_transaction(w http.ResponseWriter, r *http.Request) *Transaction {
	trans_id := mux.Vars(r)["id"]
	trans, ok := store.transactions[trans_id]
	if !ok || trans == nil {
		services.Http_respond_error(w, r, "Transaction not found: "+trans_id, http.StatusNotFound)
		return nil
	}
	return trans
}

func (store *AccountStore) show_transaction(w http.ResponseWriter, r *http.Request) {
	if trans := store.get_transaction(w, r); trans != nil {
		services.Http_respond_json(w, r, trans)
	}
}

func (store *AccountStore) cancel_transaction(w http.ResponseWriter, r *http.Request) {
	if trans := store.get_transaction(w, r); trans != nil {
		if err := trans.Cancel(); err != nil {
			services.Http_application_error(w, r, err)
		}
		services.Http_respond(w, r, nil, http.StatusOK)
	}
}

func (store *AccountStore) commit_transaction(w http.ResponseWriter, r *http.Request) {
	if trans := store.get_transaction(w, r); trans != nil {
		if err := trans.Commit(); err != nil {
			services.Http_application_error(w, r, err)
			return
		}
		services.Http_respond(w, r, nil, http.StatusOK)
	}
}

func (store *AccountStore) revert_transaction(w http.ResponseWriter, r *http.Request) {
	if trans := store.get_transaction(w, r); trans != nil {
		if err := trans.Revert(); err != nil {
			services.Http_application_error(w, r, err)
			return
		}
		services.Http_respond(w, r, nil, http.StatusOK)
	}
}
