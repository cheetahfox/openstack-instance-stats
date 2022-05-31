package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/diagnostics"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

type openstackServers interface {
	updateServers() error
	populateServers() error
}

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
}

// This fucntion sets up the program func startup() *gophercloud.ProviderClient
func startup() (*gophercloud.ProviderClient, sysconfig) {
	var config sysconfig
	var missingEnv bool

	// Check that we have the require OpenStack enviroment vars set
	if os.Getenv("OS_AUTH_URL") == "" {
		log.Println("No Openstack Auth URL supplied")
		missingEnv = true
	}
	if os.Getenv("OS_USERNAME") == "" {
		log.Println("No Openstack Username supplied")
		missingEnv = true
	}
	if os.Getenv("OS_PASSWORD") == "" {
		log.Println("No Openstack Password supplied")
		missingEnv = true
	}
	if os.Getenv("OS_PROJECT_DOMAIN_ID") == "" {
		log.Println("No Openstack Project Domain ID supplied")
		missingEnv = true
	}
	if os.Getenv("OS_REGION_NAME") == "" {
		log.Println("No Region Name supplied")
		missingEnv = true
	}
	if os.Getenv("OS_PROJECT_NAME") == "" {
		log.Println("No Project Name supplied")
		missingEnv = true
	}
	if os.Getenv("OS_USER_DOMAIN_NAME") == "" {
		log.Println("No User Domain Name supplied")
		missingEnv = true
	}
	if os.Getenv("OS_INTERFACE") == "" {
		log.Println("No Interface type supplied")
		missingEnv = true
	}
	if os.Getenv("OS_PROJECT_ID") == "" {
		log.Println("No Openstack Tenant Id supplied")
		missingEnv = true
	}
	// Newer Openstack Env might not have this set, so if we have USER domain we match it
	if os.Getenv("OS_DOMAIN_NAME") == "" || os.Getenv("OS_USER_DOMAIN_NAME") != "" {
		os.Setenv("OS_DOMAIN_NAME", os.Getenv("OS_USER_DOMAIN_NAME"))
	} else if os.Getenv("OS_DOMAIN_NAME") == "" {
		log.Println("No OpenStack Domain name supplied")
		missingEnv = true
	}
	if os.Getenv("OS_REGION_NAME") == "" {
		log.Println("No OpenStack Region Name supplied")
		missingEnv = true
	}
	// Influx Enviromental Variables
	if os.Getenv("INFLUX_SERVER") == "" {
		log.Println("No Influxdb server specifed")
		missingEnv = true
	}
	if os.Getenv("INFLUX_TOKEN") == "" {
		log.Println("No Influxdb v2 Token specifed")
		missingEnv = true
	}
	if os.Getenv("INFLUX_BUCKET") == "" {
		log.Println("No Influx bucket specifed")
		missingEnv = true
	}
	if os.Getenv("INFLUX_ORG") == "" {
		log.Println("No Influx org specifed")
		missingEnv = true
	}
	// Set the config from the Env
	config.InfluxdbServer = os.Getenv("INFLUX_SERVER")
	config.Token = os.Getenv("INFLUX_TOKEN")
	config.Bucket = os.Getenv("INFLUX_BUCKET")
	config.Org = os.Getenv("INFLUX_ORG")

	// Log and exit if we are missing vars
	if missingEnv {
		log.Fatal("Missing Enviromental vars")
	}

	// Lets connect to Openstack now using these values
	opts, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		log.Fatal(err)
	}

	// Just set the refresh time to 15 seconds for now.
	config.RefreshTime = 15

	return provider, config
}

// Fill the server list for the first time
func populateServers(provider *gophercloud.ProviderClient) ([]vms, error) {
	var osServers []vms

	endpoint := gophercloud.EndpointOpts{Region: os.Getenv("OS_REGION_NAME")}
	client, err := openstack.NewComputeV2(provider, endpoint)
	if err != nil {
		return nil, err
	}

	// Get all servers
	listOpts := servers.ListOpts{
		AllTenants: false,
		Name:       "",
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

	return osServers, nil
}

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

func statsWorker(config sysconfig, instances []vms, osProvider *gophercloud.ProviderClient, dbapi api.WriteAPI) {
	ticker := time.NewTicker(time.Second * time.Duration(config.RefreshTime))
	for range ticker.C {
		for _, s := range instances {
			if s.Status == "ACTIVE" {
				stats, err := serverStats(osProvider, s.UUID)
				if err != nil {
					log.Println(err)
					fmt.Println("Error while getting Server stats")
				}
				for k, v := range stats {
					p := influxdb2.NewPointWithMeasurement("OpenStack Metrics").
						AddTag("Instance Name", s.Name).
						AddTag("UUID", s.UUID).
						AddTag("Project", s.ProjectID).
						AddField(k, v).
						SetTime(time.Now())
					dbapi.WritePoint(p)
				}
			}
		}
	}
}

func main() {
	// Check the Enviromental Vars
	osProvider, config := startup()

	/*
		Get the Instance list from openstack
		In the future we will need to dynamically update this. But for now just pull the list at startup.
	*/
	osVms, err := populateServers(osProvider)
	if err != nil {
		log.Println(err)
		log.Println("Error while populating server list")
	}

	dbclient := influxdb2.NewClient(config.InfluxdbServer, config.Token)
	writeAPI := dbclient.WriteAPI(config.Org, config.Bucket)
	errorsCh := writeAPI.Errors()
	// Catch any write errors
	go func() {
		for err := range errorsCh {
			fmt.Printf("write error: %s\n", err.Error())
		}
	}()

	// Go into the main loop.
	go statsWorker(config, osVms, osProvider, writeAPI)

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

	fmt.Println("Startup success")
	<-done
	// Close the Influxdb connection
	writeAPI.Flush()
	dbclient.Close()
	fmt.Println("exiting")

}
