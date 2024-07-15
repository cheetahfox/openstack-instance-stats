package influx

import (
	"time"
	"context"
	"fmt"
	"log"

	config "github.com/cheetahfox/openstack-instance-stats/config"
	metrics "github.com/cheetahfox/openstack-instance-stats/metrics"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/domain"
)

var writeAPI api.WriteAPI
var client influxdb2.Client 

func SetupInfluxDB(conf config.Sysconfig) {
	dbclient := influxdb2.NewClient(conf.InfluxdbServer, conf.Token)
	health, err := dbclient.Health(context.Background())
	if (err != nil) && health.Status == domain.HealthCheckStatusPass {
		log.Panic(err)
	}
	writeAPI := dbclient.WriteAPI(conf.Org, conf.Bucket)
	errorsCh := writeAPI.Errors()
	// Catch any write errors
	go func() {
		for err := range errorsCh {
			fmt.Printf("write error: %s\n", err.Error())
		}
	}()
}

// This writes a point that we have computed from the normal statistics
func WritePoint(s metrics.Vms, m string, f string, v float64, dbapi api.WriteAPI) {
	t := time.Now()
	p := influxdb2.NewPointWithMeasurement(m).
		AddTag("Instance Name", s.Name).
		AddTag("UUID", s.UUID).
		AddTag("Project", s.ProjectID).
		AddField(f, v).
		SetTime(t)
	dbapi.WritePoint(p)
	// fmt.Println("Wrote Data for "+f+" : %f : at", v, t.String())
}