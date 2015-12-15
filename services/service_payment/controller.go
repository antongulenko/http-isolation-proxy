package main

import (
	"net/http"
	"strconv"

	"github.com/antongulenko/http-isolation-proxy/services"
	"github.com/gorilla/mux"
)

func (payments *Payments) new_payment(w http.ResponseWriter, r *http.Request) {
	value, err := strconv.ParseFloat(r.FormValue("value"), 64)
	if err != nil {
		services.Http_respond_error(w, r, "Failed to parse 'value' parameter to float: "+err.Error(), http.StatusBadRequest)
		return
	}
	username := r.FormValue("user")
	timestamp := r.FormValue("ts")
	payment, existed, err := payments.NewPayment(username, value, timestamp)
	if err != nil {
		services.Http_respond_error(w, r, "Error creating payment: "+err.Error(), http.StatusInternalServerError)
	} else {
		var code int
		if existed {
			code = http.StatusOK
		} else {
			code = http.StatusCreated
		}
		services.Http_respond(w, r, ([]byte)(payment.id), code)
	}
}

func (payments *Payments) get_payment(w http.ResponseWriter, r *http.Request, lockPayment bool) *Payment {
	id := mux.Vars(r)["id"]
	payment := payments.MakePayment(id)
	existed, err := payment.LoadExisting(lockPayment, true)
	if err != nil {
		services.Http_respond_error(w, r, "Error fetching payment "+id+": "+err.Error(), http.StatusInternalServerError)
		return nil
	} else if !existed {
		services.Http_respond_error(w, r, "Payment "+id+" does not exist", http.StatusNotFound)
		return nil
	}
	return payment
}

func (payments *Payments) show_payment(w http.ResponseWriter, r *http.Request) {
	if payment := payments.get_payment(w, r, false); payment != nil {
		services.Http_respond_json(w, r, payment)
	}
}

func (payments *Payments) commit_payment(w http.ResponseWriter, r *http.Request) {
	if payment := payments.get_payment(w, r, true); payment != nil {
		if err := payment.Commit(); err != nil {
			services.Http_application_error(w, r, err)
		} else {
			services.Http_respond(w, r, nil, http.StatusOK)
		}
	}
}

func (payments *Payments) cancel_payment(w http.ResponseWriter, r *http.Request) {
	if payment := payments.get_payment(w, r, true); payment != nil {
		if err := payment.Cancel(); err != nil {
			services.Http_application_error(w, r, err)
		} else {
			services.Http_respond(w, r, nil, http.StatusOK)
		}
	}
}
