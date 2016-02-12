package proxy

import (
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/antongulenko/http-isolation-proxy/services"
)

const (
	overload_request_duration = 10 * time.Second
	overload_recovery_time    = 2 * time.Second
	online_check_interval     = 500 * time.Millisecond
	online_check_timeout      = 500 * time.Millisecond
)

type Endpoint struct {
	Service string
	Host    string

	active        bool
	overloaded    bool
	reqs          uint
	load          int
	errors        int
	lock          sync.Mutex
	totalDuration time.Duration

	activeLock    sync.Mutex
	activeWaiters []chan<- *Endpoint
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

func (endpoint *Endpoint) Overloaded() bool {
	return endpoint.overloaded
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
			defer endpoint.activeLock.Unlock()
			endpoint.activeLock.Lock()
			endpoint.setInactive(err)
		}
	}()
	err = roundTripper()
}

func (endpoint *Endpoint) backgroundCheck() {
	if endpoint.overloaded {
		// In case of overload, just wait some time
		// to let the endpoint recover from overload
		time.Sleep(overload_recovery_time)
		endpoint.activeLock.Lock()
		defer endpoint.activeLock.Unlock()
		if !endpoint.active && endpoint.overloaded {
			endpoint.setActive()
		}
	} else {
		for {
			time.Sleep(online_check_interval)
			err := endpoint.CheckConnection()
			func() { // Extra func for defer
				endpoint.activeLock.Lock()
				defer endpoint.activeLock.Unlock()
				if endpoint.active || endpoint.overloaded {
					return // Something else resolved/changed the situation
				}
				if err == nil {
					endpoint.setActive()
					return
				} else {
					services.L.Tracef("%v offline: %v", endpoint, err)
				}
			}()
		}
	}
}

func (endpoint *Endpoint) WaitActive() <-chan *Endpoint {
	result := make(chan *Endpoint, 1)
	defer endpoint.activeLock.Unlock()
	endpoint.activeLock.Lock()
	if endpoint.active {
		result <- endpoint
	} else {
		endpoint.activeWaiters = append(endpoint.activeWaiters, result)
	}
	return result
}

func (endpoint *Endpoint) CheckConnection() error {
	conn, err := net.DialTimeout("tcp", endpoint.Host, online_check_timeout)
	if conn != nil {
		_ = conn.Close()
	}
	return err
}

func (endpoint *Endpoint) TestActive() {
	err := endpoint.CheckConnection()
	defer endpoint.activeLock.Unlock()
	endpoint.activeLock.Lock()
	if err == nil {
		endpoint.setActive()
	} else {
		endpoint.setInactive(err)
	}
}

// Must be called with locked endpoint.activeLock
func (endpoint *Endpoint) setActive() {
	services.L.Warnf("%v active", endpoint)
	endpoint.active = true
	endpoint.overloaded = false
	for _, waiter := range endpoint.activeWaiters {
		waiter <- endpoint
	}
}

// Must be called with locked endpoint.activeLock
func (endpoint *Endpoint) setInactive(err error) {
	if endpoint.active {
		services.L.Warnf("%v inactive due to: %v", endpoint, err)
		endpoint.active = false
		if err == nil {
			err = endpoint.CheckConnection()
		}
		endpoint.overloaded = err == nil
		go endpoint.backgroundCheck()
	}
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
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			if interfaceAddr, ok := addr.(*net.IPNet); ok {
				if iface.Flags&net.FlagLoopback != 0 && interfaceAddr.Contains(ip.IP) {
					return port, nil
				}
				if interfaceAddr.IP.Equal(ip.IP) {
					return port, nil
				}
			}
		}
	}
	return "", nil
}
