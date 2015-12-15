package catalogApi

import (
	"fmt"
	"net/url"

	"github.com/antongulenko/http-isolation-proxy/services"
)

type Item struct {
	Name     string  `json:"name" redis:"-"`
	Stock    uint64  `json:"stock"`
	Reserved uint64  `json:"reserved"`
	Shipped  uint64  `json:"shipped"`
	Cost     float64 `json:"cost"`
}

func (item *Item) String() string {
	return fmt.Sprintf("%s (%v, %v stock, %v reserved, %v shipped)", item.Name, item.Cost, item.Stock, item.Reserved, item.Shipped)
}

type ShipmentStatus string

const (
	ShipmentCreated   = ShipmentStatus("created")
	ShipmentCommitted = ShipmentStatus("committed")
	ShipmentCancelled = ShipmentStatus("cancelled")
	ShipmentDelivered = ShipmentStatus("delivered")
)

type Shipment struct {
	User     string         `json:"user"`
	Item     string         `json:"item"`
	Quantity uint64         `json:"quantity"`
	Status   ShipmentStatus `json:"status"`
}

func (shipment *Shipment) String() string {
	return fmt.Sprintf("Shipment (%v) %vx %v -> %v", shipment.Status, shipment.Quantity, shipment.Item, shipment.User)
}

func AllItems(endpoint string) ([]*Item, error) {
	var result []*Item
	return result, services.Http_get_json("http://"+endpoint+"/items", &result)
}

func GetItem(endpoint string, item string) (*Item, error) {
	var result Item
	return &result, services.Http_get_json("http://"+endpoint+"/item/"+item, &result)
}

func ShipItem(endpoint string, item string, user string, quantity uint64, timestamp string) (string, error) {
	return services.Http_post_string("http://"+endpoint+"/item/"+item+"/ship",
		url.Values{
			"user": []string{user},
			"qty":  []string{fmt.Sprintf("%v", quantity)},
			"ts":   []string{timestamp},
		})
}

func GetShipment(endpoint string, id string) (*Shipment, error) {
	var result Shipment
	return &result, services.Http_get_json("http://"+endpoint+"/shipment/"+id, &result)
}

func CommitShipment(endpoint string, id string) error {
	return services.Http_simple_post("http://" + endpoint + "/shipment/" + id + "/commit")
}

func CancelShipment(endpoint string, id string) error {
	return services.Http_simple_post("http://" + endpoint + "/shipment/" + id + "/cancel")
}

func DeliverShipment(endpoint string, id string) error {
	return services.Http_simple_post("http://" + endpoint + "/shipment/" + id + "/deliver")
}
