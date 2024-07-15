package metrics

import (
	"net"
)

type Vms struct {
	UUID      string
	Name      string
	ProjectID string
	IP        net.IP
	Status    string
}