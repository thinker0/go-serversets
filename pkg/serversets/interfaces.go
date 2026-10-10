package serversets

// ZKRecord defines the APIs an endpoint record in ZooKeeper should provide.
type ZKRecord interface {
	// Marshal encodes data to be stored in ZooKeeper.
	Marshal() ([]byte, error)
	// Endpoint returns the endpoint as a host:port string.
	Endpoint() string
	// IsAlive returns true if the znode has liveness information stored.
	IsAlive() bool
}

// ZKFmt defines the APIs for endpoint's ZooKeeper path and data formats.
type ZKFmt interface {
	// Path returns the ZooKeeper full path to service members.
	Path() string
	// Prefix returns the znode name prefix for service members.
	Prefix() string
	// Create creates a new endpoint record from host and port.
	Create(host string, port int) ZKRecord
	// Unmarshal decodes encoded endpoint data into a ZKRecord.
	Unmarshal([]byte) (ZKRecord, error)
}
