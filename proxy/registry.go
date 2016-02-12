package proxy

import "fmt"

type Registry interface {
	Endpoints(serviceName string) (endpoints EndpointCollection, err error)
	Services() []string
}

type EndpointCollection []*Endpoint

func (col EndpointCollection) Get() *Endpoint {
	// Linear "search" for small collections
	var result *Endpoint
	for _, endpoint := range col {
		if endpoint.Active() {
			// Balance based on current load and history of requests (round robin in low-load situations)
			if result == nil || endpoint.Load() < result.Load() || endpoint.Reqs() < result.Reqs() {
				result = endpoint
			}
		}
	}
	return result
}

func (col EndpointCollection) EmergencyGet() *Endpoint {
	// Like Get(), but also include overloaded endpoints
	// TODO whether this should be done depends on situation on endpoint and semantics of the call
	var result *Endpoint
	for _, endpoint := range col {
		if endpoint.Active() || endpoint.Overloaded() {
			// Balance based on current load and history of requests (round robin in low-load situations)
			if result == nil || endpoint.Load() < result.Load() || endpoint.Reqs() < result.Reqs() {
				result = endpoint
			}
		}
	}
	return result
}

// Alternative implementation would use a centralized registry server
type LocalRegistry map[string]EndpointCollection

func (reg LocalRegistry) Add(serviceName string, endpoint *Endpoint) {
	reg[serviceName] = append(reg[serviceName], endpoint)
}

func (reg LocalRegistry) Endpoints(serviceName string) (endpoints EndpointCollection, err error) {
	if endpoints, ok := reg[serviceName]; ok && len(endpoints) > 0 {
		return endpoints, nil
	}
	return nil, fmt.Errorf("No endpoints registered for %s", serviceName)
}

func (reg LocalRegistry) Services() []string {
	services := make([]string, 0, len(reg))
	for service := range reg {
		services = append(services, service)
	}
	return services
}
