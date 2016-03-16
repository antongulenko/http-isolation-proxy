package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/antongulenko/golib"
	"github.com/antongulenko/http-isolation-proxy/proxy"
	"github.com/antongulenko/http-isolation-proxy/services"
	"github.com/go-ini/ini"
	"github.com/kardianos/osext"
)

const (
	stats_path   = "/stats"
	runtime_path = "/runtime"
	open_files   = 40000
)

func check(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func loadServiceRegistry(confIni *ini.File) proxy.LocalRegistry {
	reg := make(proxy.LocalRegistry)
	addService := func(name string, endpoints ...string) {
		for _, addr := range endpoints {
			endpoint := &proxy.Endpoint{
				Service: name,
				Host:    addr,
			}
			endpoint.TestActive()
			reg.Add(name, endpoint)
		}
	}

	confSection, err := confIni.GetSection("backends")
	check(err)
	for _, service := range confSection.Keys() {
		addService(service.Name(), service.Strings(",")...)
	}
	return reg
}

func isRunningLocally(service string, serviceEndpoint string, reg proxy.Registry) bool {
	if endpoints, err := reg.Endpoints(service); err == nil {
		for _, endpoint := range endpoints {
			localPort, err := endpoint.LocalPort()
			check(err)
			if localPort != "" {
				_, port, err := net.SplitHostPort(serviceEndpoint)
				check(err)
				if port == localPort {
					return true
				}
			}
		}
	}
	return false
}

func handleServices(confIni *ini.File, p *proxy.IsolationProxy) {
	confSection, err := confIni.GetSection("services")
	check(err)
	for _, service := range confSection.Keys() {
		if !isRunningLocally(service.Name(), service.String(), p.Registry) {
			go func(service *ini.Key) {
				check(p.Handle(service.Name(), service.String()))
			}(service)
		} else {
			// If the service should be running locally on the same port, don't proxy it
			services.L.Warnf("Not handling %s on %s: should be running locally", service.Name(), service.String())
		}
	}
}

func main() {
	execFolder, err := osext.ExecutableFolder()
	check(err)
	configFile := flag.String("conf", execFolder+"/isolator.ini", "Config containing isolated external services")
	statsAddr := flag.String("stats", ":7777", "Address to serve statistics (HTTP+JSON on "+stats_path+" and "+runtime_path+")")
	dialTimeout := flag.Duration("timeout", 5*time.Second, "Timeout for outgoing TCP connections")
	flag.Parse()
	golib.ConfigureOpenFilesLimit()

	confIni, err := ini.Load(*configFile)
	check(err)

	p := proxy.NewIsolationProxy(
		loadServiceRegistry(confIni),
		*dialTimeout,
	)
	services.EnableResponseLogging()
	p.ServeStats(stats_path)
	proxy.ServeRuntimeStats(runtime_path)
	handleServices(confIni, p)
	check(http.ListenAndServe(*statsAddr, nil))
}
