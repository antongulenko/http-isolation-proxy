package main

import (
	"net/http"
	"strconv"

	"github.com/antongulenko/http-isolation-proxy/services"
	"github.com/gorilla/mux"
)

func (catalog *Catalog) show_items(w http.ResponseWriter, r *http.Request) {
	if items, err := catalog.GetAllItems(); err != nil {
		services.Http_respond_error(w, r, "Error retrieving all items: "+err.Error(), http.StatusInternalServerError)
	} else {
		services.Http_respond_json(w, r, items)
	}
}

func (catalog *Catalog) get_item(w http.ResponseWriter, r *http.Request) *Item {
	item_name := mux.Vars(r)["name"]
	item, err := catalog.GetItem(item_name)
	if err != nil {
		services.Http_respond_error(w, r, "Failed to retrieve item: "+err.Error(), http.StatusInternalServerError)
		return nil
	} else if item == nil {
		services.Http_respond_error(w, r, "Item does not exist: "+item_name, http.StatusNotFound)
		return nil
	}
	return item
}

func (catalog *Catalog) show_item(w http.ResponseWriter, r *http.Request) {
	if item := catalog.get_item(w, r); item != nil {
		services.Http_respond_json(w, r, item)
	}
}

func (catalog *Catalog) ship_item(w http.ResponseWriter, r *http.Request) {
	item := catalog.get_item(w, r)
	if item == nil {
		return
	}
	username := r.FormValue("user")
	timestamp := r.FormValue("ts")
	qtyString := r.FormValue("qty")
	qty, err := strconv.ParseUint(qtyString, 10, 64)
	if err != nil {
		services.Http_respond_error(w, r, "Failed to parse 'qty' parameter to uint: "+err.Error(), http.StatusBadRequest)
		return
	}
	if shipment, existed, err := item.Ship(username, qty, timestamp); err != nil {
		services.Http_respond_error(w, r, "Failed to ship item: "+err.Error(), http.StatusInternalServerError)
	} else {
		var code int
		if existed {
			code = http.StatusOK
		} else {
			code = http.StatusCreated
		}
		services.Http_respond(w, r, ([]byte)(shipment.id), code)
	}
}

func (catalog *Catalog) get_shipment(w http.ResponseWriter, r *http.Request) *Shipment {
	shipment_id := mux.Vars(r)["id"]
	shipment := catalog.MakeShipment(shipment_id)
	existed, err := shipment.LoadExisting()
	if err != nil {
		services.Http_respond_error(w, r, "Failed to retrieve shipment: "+err.Error(), http.StatusInternalServerError)
		return nil
	} else if !existed {
		services.Http_respond_error(w, r, "Shipment does not exist: "+shipment_id, http.StatusNotFound)
		return nil
	}
	return shipment
}

func (catalog *Catalog) show_shipment(w http.ResponseWriter, r *http.Request) {
	if shipment := catalog.get_shipment(w, r); shipment != nil {
		services.Http_respond_json(w, r, shipment)
	}
}

func (catalog *Catalog) commit_shipment(w http.ResponseWriter, r *http.Request) {
	if shipment := catalog.get_shipment(w, r); shipment != nil {
		if err := shipment.Commit(); err != nil {
			services.Http_application_error(w, r, err)
		} else {
			services.Http_respond(w, r, nil, http.StatusOK)
		}
	}
}

func (catalog *Catalog) cancel_shipment(w http.ResponseWriter, r *http.Request) {
	if shipment := catalog.get_shipment(w, r); shipment != nil {
		if err := shipment.Cancel(); err != nil {
			services.Http_application_error(w, r, err)
		} else {
			services.Http_respond(w, r, nil, http.StatusOK)
		}
	}
}

func (catalog *Catalog) deliver_shipment(w http.ResponseWriter, r *http.Request) {
	if shipment := catalog.get_shipment(w, r); shipment != nil {
		if err := shipment.Deliver(); err != nil {
			services.Http_application_error(w, r, err)
		} else {
			services.Http_respond(w, r, nil, http.StatusOK)
		}
	}
}
