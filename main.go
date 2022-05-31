package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/diagnostics"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
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
	Active    bool
}

type sysconfig struct {
	InfluxdbServer string
	InfluxDB       string
	InfluxUsername string
	InfluxPassword string
}

// This fucntion sets up the program func startup() *gophercloud.ProviderClient
func startup() (*gophercloud.ProviderClient, sysconfig) {
	var config sysconfig
	var missingEnv bool

	// Check that we have the require enviroment vars set
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
	if missingEnv == true {
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

	return provider, config
}

/*
func (servers []vms) updateServers(error) {

}
*/

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
		if server.Status == "ACTIVE" {
			s.Active = true
		}

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

func main() {
	osProvider, config := startup()

	spew.Dump(config)

	osVms, err := populateServers(osProvider)
	if err != nil {
		log.Println(err)
		log.Println("Error while populating server list")
	}

	for _, s := range osVms {
		fmt.Println("-------------------------")
		fmt.Printf("Server name: %s\n", s.Name)
		fmt.Printf("Server UUID: %s\n", s.UUID)
		fmt.Printf("Project ID: %s\n", s.ProjectID)
		fmt.Println(s.Active)
		stats, err := serverStats(osProvider, s.UUID)
		if err != nil {
			log.Println(err)
			fmt.Println("Error while getting Server stats")
		}

		spew.Dump(stats)
	}
	/*
		opts := gophercloud.EndpointOpts{Region: os.Getenv("OS_REGION_NAME")}

		client, err := openstack.NewComputeV2(provider, opts)
	*/
}
