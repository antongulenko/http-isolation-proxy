package shopApi

import (
	"fmt"
	"net/url"

	"github.com/antongulenko/http-isolation-proxy/services"
	"github.com/antongulenko/http-isolation-proxy/services/service_catalog/catalogApi"
)

type Item catalogApi.Item

type Order struct {
	User     string `json:"user"`
	Item     string `json:"item"`
	Quantity uint64 `json:"quantity"`
	Status   string `json:"status"`
	Time     string `json:"time"`
}

func AllItems(shopEndpoint string) ([]*Item, error) {
	var result []*Item
	return result, services.Http_get_json("http://"+shopEndpoint+"/shop", &result)
}

func AllOrders(shopEndpoint string, user string) ([]*Order, error) {
	var result []*Order
	return result, services.Http_get_json("http://"+shopEndpoint+"/orders/"+user, &result)
}

func PlaceOrder(shopEndpoint string, user string, item string, quantity int64) error {
	_, err := services.Http_post_string("http://"+shopEndpoint+"/order",
		url.Values{
			"user": []string{user},
			"item": []string{item},
			"qty":  []string{fmt.Sprintf("%v", quantity)},
		})
	return err
}
