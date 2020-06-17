package serversets

import (
	"encoding/json"
	"net"
	"strconv"
)

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

type FinagleRecordProvider struct{}

// Create creates an alive endpoint record from host and port.
func (FinagleRecordProvider) Create(host string, port int) ZKRecord {
	return &FinagleRecord{
		ServiceEndpoint:     endpoint{host, port},
		AdditionalEndpoints: make(map[string]endpoint),
		Shard:               0,
		Status:              statusAlive,
	}
}

// Unmarshal decodes zk record from JSON format.
func (FinagleRecordProvider) Unmarshal(data []byte) (ZKRecord, error) {
	f := &FinagleRecord{}
	err := json.Unmarshal(data, f)
	return f, err
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
