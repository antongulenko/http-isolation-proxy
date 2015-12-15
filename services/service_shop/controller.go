package main

import (
	"net/http"
	"strconv"

	"github.com/antongulenko/http-isolation-proxy/services"
	"github.com/gorilla/mux"
)

func (shop *Shop) show_items(w http.ResponseWriter, r *http.Request) {
	if items, err := shop.AllItems(); items != nil {
		// TODO instead of parsing and encoding the JSON reply, simply forward it
		services.Http_respond_json(w, r, items)
	} else {
		services.Http_respond_error(w, r, "Failed to fetch item list: "+err.Error(), http.StatusInternalServerError)
	}
}

func (shop *Shop) order_item(w http.ResponseWriter, r *http.Request) {
	qty, err := strconv.ParseUint(r.FormValue("qty"), 10, 64)
	if err != nil {
		services.Http_respond_error(w, r, "Failed to parse 'qty' parameter to uint: "+err.Error(), http.StatusBadRequest)
		return
	}
	username := r.FormValue("user")
	item := r.FormValue("item")
	if err := shop.NewOrder(username, item, qty); err != nil {
		services.Http_respond_error(w, r, "Error creating order: "+err.Error(), http.StatusInternalServerError)
	} else {
		services.Http_respond(w, r, nil, http.StatusCreated)
	}
}

func (shop *Shop) show_orders(w http.ResponseWriter, r *http.Request) {
	username := mux.Vars(r)["user"]
	if orders, err := shop.AllOrders(username); orders != nil {
		services.Http_respond_json(w, r, orders)
	} else {
		services.Http_respond_error(w, r, "Failed to fetch orders: "+err.Error(), http.StatusInternalServerError)
	}
}
