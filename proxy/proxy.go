package proxy

import (
	"errors"
	"net/http"
	"net/http/httputil"

	"github.com/antongulenko/http-isolation-proxy/services"
)

var (
	noActiveEndpointsErr = errors.New("No active endpoints")
)

type IsolationProxy struct {
	Registry Registry
}

type Director struct {
	proxy       *IsolationProxy
	serviceName string
}

func (proxy *IsolationProxy) Handle(serviceName, localEndpoint string) error {
	director := &Director{
		proxy:       proxy,
		serviceName: serviceName,
	}
	p := &httputil.ReverseProxy{
		Director:  director.direct,
		Transport: director,
	}
	return http.ListenAndServe(localEndpoint, p)
}

func (director *Director) direct(req *http.Request) {
	if req.URL.Scheme == "" {
		req.URL.Scheme = "http"
	}
	// Don't do anything here since we also define the RoundTripper
}

func (director *Director) endpointFor(req *http.Request) (*Endpoint, error) {
	endpoints, err := director.proxy.Registry.Endpoints(director.serviceName)
	if err == nil {
		endpoint := endpoints.Get()
		if endpoint != nil {
			return endpoint, nil
		}
		err = noActiveEndpointsErr
	}
	return nil, err
}

func (director *Director) serviceUnavailable(req *http.Request) *http.Response {
	return services.MakeHttpResponse(req, http.StatusServiceUnavailable,
		"No server available to handle your request\n")
}

func (director *Director) RoundTrip(req *http.Request) (*http.Response, error) {
	if endpoint, err := director.endpointFor(req); err != nil {
		services.L.Logf("Cannot forward %s request for %s: %v", director.serviceName, req.URL.Path, err)
		return director.serviceUnavailable(req), nil
	} else {
		services.L.Logf("Forwarding %s to %v for %s", director.serviceName, endpoint, req.URL.Path)
		endpoint.ConfigureUrl(req.URL)
		var resp *http.Response
		endpoint.RoundTrip(func() error {
			resp, err = http.DefaultTransport.RoundTrip(req)
			return err
		})
		if err != nil {
			services.L.Warnf("Error forwarding %s to %v for %s: %v. Trying again...", director.serviceName, endpoint, req.URL.Path, err)
			return director.RoundTrip(req) // Should pick a different endpoint
		}
		return resp, err
	}
}
