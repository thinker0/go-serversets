package serversets

// ZKRecord defines the APIs an endpoint record in zookeeper should provide.
type ZKRecord interface {
	// Marshal encodes data to be stored in zookeeper.
	Marshal() ([]byte, error)
	// Endpoint returns the endpoint as host:port string
	Endpoint() string
	// IsAlive returns true if znode has liveness information stored.
	IsAlive() bool
}

// ZKFmt defines the APIs for endpoint's zookeeper path and data formats.
type ZKFmt interface {
	// Path returns the zokeeper full path to service members.
	Path() string
	// Prefix returns the znode name prefix for service members.
	Prefix() string
	// CreateRecord creates a new endpoint record from host and port.
	Create(host string, port int) ZKRecord
	// Unmarshal decodes encoded endpoint data a ZKrecord.
	Unmarshal([]byte) (ZKRecord, error)
}
