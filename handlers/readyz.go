package handlers

import (
	"context"
	"net/http"
	"sync/atomic"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/domain"
)

// Ready Check where we look at the
func readyz(isReady *atomic.Value, dbclient influxdb2.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		// Check database Status. If unhealthy return a http error status
		health, err := dbclient.Health(context.Background())
		if (err != nil) || health.Status != domain.HealthCheckStatusPass {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
		if isReady == nil || !isReady.Load().(bool) {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
	}
}
