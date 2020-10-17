package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
)

// This fucntion sets up the program func startup() *gophercloud.ProviderClient
func startup() {
	// Check that we have the require enviroment vars set
	if os.Getenv("OS_AUTH_URL") == "" {
		log.Fatal("No Openstack Auth URL supplied")
	}
	if os.Getenv("OS_USERNAME") == "" {
		log.Fatal("No Openstack Username supplied")
	}
	if os.Getenv("OS_PASSWORD") == "" {
		log.Fatal("No Openstack Password supplied")
	}
	if os.Getenv("OS_PROJECT_DOMAIN_ID") == "" {
		log.Fatal("No Openstack Project Domain ID supplied")
	}
	if os.Getenv("OS_REGION_NAME") == "" {
		log.Fatal("No Region Name supplied")
	}
	if os.Getenv("OS_PROJECT_NAME") == "" {
		log.Fatal("No Project Name supplied")
	}
	if os.Getenv("OS_USER_DOMAIN_NAME") == "" {
		log.Fatal("No User Domain Name supplied")
	}
	if os.Getenv("OS_INTERFACE") == "" {
		log.Fatal("No Interface type supplied")
	}
	if os.Getenv("OS_PROJECT_ID") == "" {
		log.Fatal("No Openstack Tenant Id supplied")
	}
	if os.Getenv("OS_DOMAIN_NAME") == "" {
		log.Fatal("No OpenStack Domain name supplied")
	}
	fmt.Println("Good Enviormental Values")

	// Lets connect to Openstack now using these values
	opts, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		log.Fatal(err)
	}

	endpoint := gophercloud.EndpointOpts{Region: os.Getenv("OS_REGION_NAME")}
	client, err := openstack.NewComputeV2(provider, endpoint)

	listOpts := servers.ListOpts{
		AllTenants: true,
	}

	allPages, err := servers.List(client, listOpts).AllPages()
	if err != nil {
		panic(err)
	}

	allServers, err := servers.ExtractServers(allPages)
	if err != nil {
		panic(err)
	}

	for _, server := range allServers {
		fmt.Printf("|%+v  | %+v\n", server.Name, server.ID)
	}

	//return provider
}

func main() {
	//provider := startup()
	startup()

	/*
		opts := gophercloud.EndpointOpts{Region: os.Getenv("OS_REGION_NAME")}

		client, err := openstack.NewComputeV2(provider, opts)
	*/
}
