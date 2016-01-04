package proxy

import (
	"net/http"
	"runtime"

	"github.com/antongulenko/http-isolation-proxy/services"
)

type RuntimeStats struct {
	Goroutines int
}

func HandleRuntimeStats() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stats := GetRuntimeStats()
		services.Http_respond_json(w, r, stats)
		return
	})
}

func ServeRuntimeStats(pattern string) {
	http.Handle(pattern, HandleRuntimeStats())
}

func GetRuntimeStats() (result RuntimeStats) {
	result.Goroutines = runtime.NumGoroutine()
	return
}
