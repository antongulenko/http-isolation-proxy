package proxy

import (
	"net/http"
	"time"

	"github.com/antongulenko/http-isolation-proxy/services"
)

type Stats struct {
	Requests    uint
	Load        int
	AvgDuration string
	Active      bool
	Errors      int

	totalDuration time.Duration
}

type EndpointStats struct {
	Stats
	Endpoints map[string]Stats
}

type ProxyStats map[string]*EndpointStats

func (proxy *IsolationProxy) HandleStats() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stats := proxy.Stats()
		services.Http_respond_json(w, r, stats)
		return
	})
}

func (proxy *IsolationProxy) ServeStats(pattern string) {
	http.Handle(pattern, proxy.HandleStats())
}

func (proxy *IsolationProxy) Stats() ProxyStats {
	result := make(ProxyStats)
	for _, service := range proxy.Registry.Services() {
		stats := &EndpointStats{
			Endpoints: make(map[string]Stats),
		}
		endpoints, err := proxy.Registry.Endpoints(service)
		if err != nil {
			continue
		}
		for _, endpoint := range endpoints {
			stats.fillFrom(endpoint)
			eStats := Stats{}
			eStats.fillFrom(endpoint)
			eStats.compute()
			stats.Endpoints[endpoint.Name()] = eStats
		}
		stats.compute()
		result[service] = stats
	}
	return result
}

func (stats *Stats) fillFrom(endpoint *Endpoint) {
	stats.Requests += endpoint.Reqs()
	stats.Load += endpoint.Load()
	stats.totalDuration += endpoint.totalDuration
	stats.Active = stats.Active || endpoint.Active()
	stats.Errors += endpoint.Errors()
}

func (stats *Stats) compute() {
	if stats.Requests == 0 {
		stats.AvgDuration = "(no data)"
	} else {
		avg := stats.totalDuration / time.Duration(stats.Requests)
		stats.AvgDuration = avg.String()
	}
}
