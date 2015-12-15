package paymentApi

import (
	"fmt"
	"net/url"

	"github.com/antongulenko/http-isolation-proxy/services"
)

const (
	PaymentCreated   = "created"
	PaymentPending   = "pending"
	PaymentCommitted = "commited"
	PaymentProcessed = "processed"
	PaymentFailed    = "failed"
	PaymentCancelled = "cancelled"
)

type Payment struct {
	User   string  `json:"user"`
	Value  float64 `json:"value"`
	Error  string  `json:"error" redis:"-"`
	Status string  `json:"status" redis:"-"`
}

func (p *Payment) String() string {
	errStr := p.Error
	if errStr != "" {
		errStr = ": " + errStr
	}
	return fmt.Sprintf("Payment (%v) %v from %v", p.Status, p.Value, p.User, errStr)
}

func CreatePayment(endpoint string, user string, value float64, timestamp string) (string, error) {
	return services.Http_post_string("http://"+endpoint+"/payment",
		url.Values{
			"user":  []string{user},
			"value": []string{fmt.Sprintf("%v", value)},
			"ts":    []string{timestamp},
		})
}

func FetchPayment(endpoint string, id string) (*Payment, error) {
	var result Payment
	return &result, services.Http_get_json("http://"+endpoint+"/payment/"+id, &result)
}

func CommitPayment(endpoint string, id string) error {
	return services.Http_simple_post("http://" + endpoint + "/payment/" + id + "/commit")
}

func CancelPayment(endpoint string, id string) error {
	return services.Http_simple_post("http://" + endpoint + "/payment/" + id + "/cancel")
}
