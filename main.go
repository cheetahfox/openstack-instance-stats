package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"regexp"
	"syscall"
	"time"

	"github.com/cheetahfox/openstack-instance-stats/handlers"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/diagnostics"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/domain"
)

type vms struct {
	UUID      string
	Name      string
	ProjectID string
	IP        net.IP
	Status    string
}

type sysconfig struct {
	Bucket         string
	InfluxdbServer string
	Org            string
	Token          string
	RefreshTime    int
	WebPort        string
	Scope          string
}

// This fucntion sets up the program func startup() *gophercloud.ProviderClient
func startup() (*gophercloud.ProviderClient, sysconfig) {
	var config sysconfig

	// Required Enviorment vars mostly OpenStack Env vars.
	requiredEnvVars := []string{
		"OS_AUTH_URL",
		"OS_USERNAME",
		"OS_PASSWORD",
		"OS_PROJECT_DOMAIN_ID",
		"OS_REGION_NAME",
		"OS_PROJECT_NAME",
		"OS_USER_DOMAIN_NAME",
		"OS_INTERFACE",
		"OS_PROJECT_ID",
		"OS_DOMAIN_NAME",
		"OS_REGION_NAME",
		"INFLUX_SERVER", // Influxdb server url including port number
		"INFLUX_TOKEN",  // Influx Token
		"INFLUX_BUCKET", // Influx bucket
		"INFLUX_ORG",    // Influx ord
		"STATS_PORT",    // port number for the kubernetes checks
		"SCOPE",         // "site" or "project"; get stats on ALL instances or just a single project
	}

	// Newer Openstack Env might not have this set, so if we have USER domain we match it
	if os.Getenv("OS_DOMAIN_NAME") == "" || os.Getenv("OS_USER_DOMAIN_NAME") != "" {
		os.Setenv("OS_DOMAIN_NAME", os.Getenv("OS_USER_DOMAIN_NAME"))
	}

	// Check if the Required Enviromental varibles are set exit if they aren't.
	for index := range requiredEnvVars {
		if os.Getenv(requiredEnvVars[index]) == "" {
			log.Fatalf("Missing %s Enviroment var \n", requiredEnvVars[index])
		}
	}

	// Set the config from the Env
	config.WebPort = os.Getenv("STATS_PORT")
	config.InfluxdbServer = os.Getenv("INFLUX_SERVER")
	config.Token = os.Getenv("INFLUX_TOKEN")
	config.Bucket = os.Getenv("INFLUX_BUCKET")
	config.Org = os.Getenv("INFLUX_ORG")
	config.Scope = os.Getenv("SCOPE")

	provider, err := osAuth()
	if err != nil {
		fmt.Println("Error while Authenticating with OpenStack for the first time.")
		log.Fatal(err)
	}

	// Just set the refresh time to 15 seconds for now.
	config.RefreshTime = 15

	return provider, config
}

// Fill the server list for the first time
func populateServers(provider *gophercloud.ProviderClient, config sysconfig) ([]vms, error) {
	var osServers []vms

	endpoint := gophercloud.EndpointOpts{Region: os.Getenv("OS_REGION_NAME")}
	client, err := openstack.NewComputeV2(provider, endpoint)
	if err != nil {
		return nil, err
	}

	listOpts := servers.ListOpts{
		AllTenants: false,
		Name:       "",
	}
	// If we are doing a site wide scan
	if config.Scope == "site" {
		listOpts.AllTenants = true
	}

	allPages, err := servers.List(client, listOpts).AllPages()
	if err != nil {
		return nil, err
	}
	allServers, err := servers.ExtractServers(allPages)
	if err != nil {
		return nil, err
	}

	var s vms

	for _, server := range allServers {
		s.UUID = server.ID
		s.Name = server.Name
		s.ProjectID = server.TenantID
		s.Status = server.Status
		osServers = append(osServers, s)
	}

	/*
		found := fmt.Sprintf("Found %d OpenStack instances", len(osServers))
		fmt.Println(found)
	*/
	return osServers, nil
}

/*
Get the Nova API Diagnostics for a specific Instance ID
*/
func serverStats(provider *gophercloud.ProviderClient, serverId string) (map[string]interface{}, error) {
	endpoint := gophercloud.EndpointOpts{Region: os.Getenv("OS_REGION_NAME")}
	client, err := openstack.NewComputeV2(provider, endpoint)
	if err != nil {
		return nil, err
	}

	diags, err := diagnostics.Get(client, serverId).Extract()
	if err != nil {
		return nil, err
	}

	return diags, nil
}

/*
Authenticate using the Enviromental vars
Return ProviderClient and err
*/
func osAuth() (*gophercloud.ProviderClient, error) {
	// Lets connect to Openstack now using these values
	opts, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	// This is super important, because the token will expire.
	opts.AllowReauth = true

	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		log.Fatal(err)
	}

	r := provider.GetAuthResult()
	if r == nil {
		return nil, errors.New("no valid auth result")
	}
	return provider, err
}

/*
statsWorker is the main data collection loop.
We get a list of current vms running and then call nova diags API to get detailed
stats about each vm.
*/
func statsWorker(config sysconfig, osProvider *gophercloud.ProviderClient, dbapi api.WriteAPI) {
	ticker := time.NewTicker(time.Second * time.Duration(config.RefreshTime))
	for range ticker.C {
		// It's only one more api call to refresh the instances every time through
		instances, err := populateServers(osProvider, config)
		if err != nil {
			log.Println(err)
			log.Println("Error while populating server list")
		}
		for _, s := range instances {
			// Only get stats from Active instances.
			if s.Status == "ACTIVE" {
				stats, err := serverStats(osProvider, s.UUID)
				if err != nil {
					log.Println(err)
					fmt.Println("Error while getting Server stats")
				}
				// Loop through the stats and write a point for each metric
				for k, v := range stats {
					val, err := getFloat(v)
					if err == nil {
						writePoint(s, "OpenStack Metrics", k, val, dbapi)
					}
				}

				// Generated metrics
				err = cpuStats(s, stats, dbapi)
				if err != nil {
					log.Println(err)
				}
				err = ioStats(s, stats, dbapi)
				if err != nil {
					log.Println(err)
				}
			}
		}
	}
}

// Sum up the CPU totals and write it out... Using legacy metric name. (I was dumb)
func cpuStats(server vms, stats map[string]interface{}, dbapi api.WriteAPI) error {
	// use this to match on CPU keys
	re, _ := regexp.Compile("cpu[0-9]+_time$")
	var cpu_total float64

	for k, v := range stats {
		if re.MatchString(k) {
			cpu_value, err := getFloat(v)
			if err != nil {
				return err
			}
			cpu_total = cpu_total + cpu_value
		}
	}

	writePoint(server, "OpenStack Metrics", "cpu_total", cpu_total, dbapi)
	return nil
}

// Function to accumulate various disk IO statistics on a instance VM
func ioStats(server vms, stats map[string]interface{}, dbapi api.WriteAPI) error {
	// vdX device io requests and
	vdr, _ := regexp.Compile("vd.+read_req$")
	vdw, _ := regexp.Compile("vd.+write_req$")
	hdr, _ := regexp.Compile("hd.+read_req$")
	hdw, _ := regexp.Compile("hd.+write_req$")
	var vdr_total, vdw_total, hdr_total, hdw_total, ior, iow float64

	for k, v := range stats {
		// Get vd* read/write ops
		if vdr.MatchString(k) {
			vdr_value, err := getFloat(v)
			if err != nil {
				return err
			}
			vdr_total = vdr_total + vdr_value
		}
		if vdw.MatchString(k) {
			vdw_value, err := getFloat(v)
			if err != nil {
				return err
			}
			vdw_total = vdw_total + vdw_value
		}

		// Get legacy hd* read/write ops
		if hdr.MatchString(k) {
			hdr_value, err := getFloat(v)
			if err != nil {
				return err
			}
			hdr_total = hdr_total + hdr_value
		}
		if hdw.MatchString(k) {
			hdw_value, err := getFloat(v)
			if err != nil {
				return err
			}
			hdw_total = hdw_total + hdw_value
		}
	}

	// Sum everything up!
	ior = vdr_total + hdr_total
	iow = vdw_total + hdw_total

	writePoint(server, "OpenStack disk", "vd_read_ops", vdr_total, dbapi)
	writePoint(server, "OpenStack disk", "vd_write_ops", vdw_total, dbapi)
	writePoint(server, "OpenStack disk", "hd_read_ops", hdr_total, dbapi)
	writePoint(server, "OpenStack disk", "hd_write_ops", hdw_total, dbapi)
	writePoint(server, "OpenStack disk", "total_read_ops", ior, dbapi)
	writePoint(server, "OpenStack disk", "total_write_ops", iow, dbapi)

	return nil
}

// This writes a point that we have computed from the normal statistics
func writePoint(s vms, m string, f string, v float64, dbapi api.WriteAPI) {
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

func getFloat(unk interface{}) (float64, error) {
	var floatType = reflect.TypeOf(float64(0))
	v := reflect.ValueOf(unk)
	v = reflect.Indirect(v)
	if !v.Type().ConvertibleTo(floatType) {
		return 0, fmt.Errorf("cannot convert %v to float64", v.Type())
	}
	fv := v.Convert(floatType)
	return fv.Float(), nil
}

func main() {
	// Check the Enviromental Vars
	osProvider, config := startup()

	// Setup the Database connection
	dbclient := influxdb2.NewClient(config.InfluxdbServer, config.Token)
	health, err := dbclient.Health(context.Background())
	if (err != nil) && health.Status == domain.HealthCheckStatusPass {
		log.Panic(err)
	}
	writeAPI := dbclient.WriteAPI(config.Org, config.Bucket)
	errorsCh := writeAPI.Errors()
	// Catch any write errors
	go func() {
		for err := range errorsCh {
			fmt.Printf("write error: %s\n", err.Error())
		}
	}()

	r := handlers.Router(dbclient)

	srv := &http.Server{
		Addr:    ":" + config.WebPort,
		Handler: r,
	}

	go func() {
		srv.ListenAndServe()
	}()

	// Go into the main loop.
	go statsWorker(config, osProvider, writeAPI)

	// Listen for Sigint or SigTerm and exit if you get them.
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		fmt.Println()
		fmt.Println(sig)
		done <- true
	}()

	fmt.Println("Startup success v0.95")

	<-done
	// Close the Influxdb connection
	writeAPI.Flush()
	dbclient.Close()
	// Shudown the webserver
	srv.Shutdown(context.Background())
	fmt.Println("exiting")
}
