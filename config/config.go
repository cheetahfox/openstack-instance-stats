package config

import (
	"fmt"
	"log"
	"os"

	"github.com/pkg/errors"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
)


type Sysconfig struct {
	Bucket         string
	InfluxdbServer string
	Org            string
	Token          string
	RefreshTime    int
	WebPort        string
	Scope          string
}


// This fucntion sets up the program func startup() *gophercloud.ProviderClient
func Startup() (*gophercloud.ProviderClient, Sysconfig) {
	var config Sysconfig

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
