package handlers

import (
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

func Router(dbclient influxdb2.Client) *mux.Router {
	isReady := &atomic.Value{}
	isReady.Store(false)

	// Startup and wait 10 seconds before checking to see if the influxDB is good
	go func() {
		time.Sleep(10 * time.Second)
		isReady.Store(true)
	}()

	r := mux.NewRouter()
	r.HandleFunc("/healthz", healthz)
	r.HandleFunc("/readyz", readyz(isReady, dbclient))

	return r
}
