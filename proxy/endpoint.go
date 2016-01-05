package proxy

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/antongulenko/http-isolation-proxy/services"
)

const (
	overload_request_duration = 1 * time.Second
	overload_recovery_time    = 2 * time.Second
	online_check_interval     = 500 * time.Millisecond
	online_check_timeout      = 500 * time.Millisecond
)

type Endpoint struct {
	Service string
	Host    string

	active        bool
	reqs          uint
	load          int
	errors        int
	lock          sync.Mutex
	totalDuration time.Duration
	onceInactive  sync.Once
}

func (endpoint *Endpoint) String() string {
	return endpoint.Service + " on " + endpoint.Host
}

func (endpoint *Endpoint) ConfigureUrl(u *url.URL) {
	u.Host = endpoint.Host
}

func (endpoint *Endpoint) Name() string {
	return endpoint.Host
}

func (endpoint *Endpoint) Load() int {
	return endpoint.load
}

func (endpoint *Endpoint) Reqs() uint {
	return endpoint.reqs
}

func (endpoint *Endpoint) Active() bool {
	return endpoint.active
}

func (endpoint *Endpoint) Errors() int {
	return endpoint.errors
}

func (endpoint *Endpoint) RoundTrip(roundTripper func() error) {
	start := time.Now()
	endpoint.lock.Lock()
	endpoint.reqs++
	endpoint.load++
	endpoint.lock.Unlock()
	var err error
	defer func() {
		duration := time.Now().Sub(start)
		endpoint.lock.Lock()
		defer endpoint.lock.Unlock()
		endpoint.load--
		endpoint.totalDuration += duration
		if duration > overload_request_duration || err != nil {
			endpoint.errors++
			endpoint.setInactive(err)
		}
	}()
	err = roundTripper()
}

func (endpoint *Endpoint) setInactive(err error) {
	endpoint.onceInactive.Do(func() {
		services.L.Warnf("%v inactive due to: %v", endpoint, err)
		endpoint.active = false
		go func() {
			defer endpoint.setActive()
			if err == nil {
				err = endpoint.CheckConnection()
			}
			if err == nil {
				// If there is no error (but request took too long), wait some time
				// to let the endpoint recover from overload
				time.Sleep(overload_recovery_time)
			} else {
				for !endpoint.active {
					time.Sleep(online_check_interval)
					if err := endpoint.CheckConnection(); err != nil {
						services.L.Tracef("%v offline: %v", endpoint, err)
						continue
					}
					break
				}

			}
		}()
	})
}

func (endpoint *Endpoint) CheckConnection() error {
	conn, err := net.DialTimeout("tcp", endpoint.Host, online_check_timeout)
	if conn != nil {
		_ = conn.Close()
	}
	return err
}

func (endpoint *Endpoint) TestActive() {
	if err := endpoint.CheckConnection(); err != nil {
		endpoint.setInactive(err)
	} else {
		endpoint.setActive()
	}
}

func (endpoint *Endpoint) setActive() {
	services.L.Warnf("%v active", endpoint)
	endpoint.active = true
	endpoint.onceInactive = sync.Once{} // Reset for next setInactive call
}

// Return empty string if the endpoint is not the local host
func (endpoint *Endpoint) LocalPort() (string, error) {
	host, port, err := net.SplitHostPort(endpoint.Host)
	if err != nil {
		return "", err
	}
	ip, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return "", err
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ipAddr, ok := addr.(*net.IPNet); ok {
			if 0 == bytes.Compare(ipAddr.IP, ip.IP) {
				return port, nil
			}
		} else {
			fmt.Printf("Type was %T\n", addr)
		}
	}
	return "", nil
}
