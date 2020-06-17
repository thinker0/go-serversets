package serversets

import (
	"encoding/json"
	"net"
	"strconv"
)

var (
	// BaseDirectory is the Zookeeper namespace that all nodes made by this package will live.
	// This path must begin with '/'
	BaseDirectory = "/aurora"

	// MemberPrefix is prefix for the Zookeeper sequential ephemeral nodes.
	// member_ is used by Finagle server sets.
	MemberPrefix = "member_"
)

// BaseZnodePath allows for a custom Zookeeper directory structure.
// This function should return the path where you want the service's members to live.
// Default is `BaseDirectory + "/" + environment + "/" + service` where the default base directory is `/aurora`
var BaseZnodePath = func(role, environment, service string) string {
	return BaseDirectory + "/" + role + "/" + environment + "/" + service
}

// FinagleFmt provides Finagle style path/data formats.
type FinagleFmt struct {
	role        string
	environment string
	service     string
}

// NewFinagleFmt creates a new FinagleFmt.
func NewFinagleFmt(role, environment, service string) FinagleFmt {
	return FinagleFmt{
		role:        role,
		environment: environment,
		service:     service,
	}
}

// Create creates an alive endpoint record from host and port.
func (FinagleFmt) Create(host string, port int) ZKRecord {
	return &FinagleRecord{
		ServiceEndpoint:     endpoint{host, port},
		AdditionalEndpoints: make(map[string]endpoint),
		Shard:               0,
		Status:              statusAlive,
	}
}

// Unmarshal decodes zk record from JSON format.
func (FinagleFmt) Unmarshal(data []byte) (ZKRecord, error) {
	f := &FinagleRecord{}
	err := json.Unmarshal(data, f)
	return f, err
}

// Path returns the znode path where all service members reside.
func (f FinagleFmt) Path() string {
	return BaseZnodePath(f.role, f.environment, f.service)
}

// Prefix returns the service member name prefix.
func (f FinagleFmt) Prefix() string {
	return MemberPrefix
}

// FinagleRecord is structure of the data in each member znode.
// It mimics finagle serverset structure.
type FinagleRecord struct {
	ServiceEndpoint     endpoint            `json:"serviceEndpoint"`
	AdditionalEndpoints map[string]endpoint `json:"additionalEndpoints"`
	Shard               int64               `json:"shard"`
	Status              string              `json:"status"`
}

type endpoint struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// Marshal encodes zk record in JSON format.
func (f *FinagleRecord) Marshal() ([]byte, error) {
	return json.Marshal(f)
}

// Endpoint returns host:port.
func (f *FinagleRecord) Endpoint() string {
	return net.JoinHostPort(f.ServiceEndpoint.Host, strconv.Itoa(f.ServiceEndpoint.Port))
}

// IsAlive returns true if this endpoint is to be discovered by user.
func (f *FinagleRecord) IsAlive() bool {
	return f.Status == statusAlive
}

// possible endpoint statuses. Currently only concerned with ALIVE.
const (
	statusDead     = "DEAD"
	statusStarting = "STARTING"
	statusAlive    = "ALIVE"
	statusStopping = "STOPPING"
	statusStopped  = "STOPPED"
	statusWarning  = "WARNING"
	statusUnknown  = "UNKNOWN"
)
