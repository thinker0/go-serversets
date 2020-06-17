package serversets

// ZKRecord defines the APIs an endpoint record in zookeeper should provide.
type ZKRecord interface {
	Marshal() ([]byte, error)
	// Endpoint returns the endpoint as host:port string
	Endpoint() string
	// IsAlive returns true if znode has liveness information stored.
	IsAlive() bool
}

// ZKRecordProvider defines the APIs for a endpoints' zookeeper record provider.
type ZKRecordProvider interface {
	Create(host string, port int) ZKRecord
	Unmarshal([]byte) (ZKRecord, error)
}
