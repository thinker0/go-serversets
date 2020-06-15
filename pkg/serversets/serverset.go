package serversets

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/samuel/go-zookeeper/zk"
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
// Default is `BaseDirectory + "/" + environment + "/" + service` where the default base directory is `/discovery`
var BaseZnodePath = func(role, environment, service string) string {
	return BaseDirectory + "/" + role + "/" + environment + "/" + service
}

// DefaultZKTimeout is the zookeeper timeout used if it is not overwritten.
var DefaultZKTimeout = 5 * time.Second

// A ServerSet represents a service with a set of servers that may change over time.
// The master lists of servers is kept as ephemeral nodes in Zookeeper.
type ServerSet struct {
	ZKTimeout   time.Duration
	role        string
	environment string
	service     string
	zkServers   []string
}

// New creates a new ServerSet object that can then be watched
// or have an endpoint added to. The service name must not contain
// any slashes. Will panic if it does.
func New(role string, environment string, service string, zookeepers []string) *ServerSet {
	if strings.Contains(service, "/") {
		panic(fmt.Errorf("service name (%s) must not contain slashes", service))
	}

	ss := &ServerSet{
		ZKTimeout:   DefaultZKTimeout,
		role:        role,
		environment: environment,
		service:     service,
		zkServers:   zookeepers,
	}

	return ss
}

// ZookeeperServers returns the Zookeeper servers this set is using.
// Useful to check if everything is configured correctly.
func (ss *ServerSet) ZookeeperServers() []string {
	return ss.zkServers
}

func (ss *ServerSet) connectToZookeeper() (*zk.Conn, <-chan zk.Event, error) {
	return zk.Connect(ss.zkServers, ss.ZKTimeout)
}

// directoryPath returns the base path of where all the ephemeral nodes will live.
func (ss *ServerSet) directoryPath() string {
	return BaseZnodePath(ss.role, ss.environment, ss.service)
}

func splitPaths(fullPath string) []string {
	var parts []string

	var last string
	for fullPath != "/" {
		fullPath, last = path.Split(path.Clean(fullPath))
		parts = append(parts, last)
	}

	// parts are in reverse order, put back together
	// into set of subdirectory paths
	result := make([]string, 0, len(parts))
	base := ""
	for i := len(parts) - 1; i >= 0; i-- {
		base += "/" + parts[i]
		result = append(result, base)
	}

	return result
}

// createFullPath makes sure all the znodes are created for the parent directories
func (ss *ServerSet) createFullPath(connection *zk.Conn) error {
	paths := splitPaths(ss.directoryPath())

	// TODO: can't we just create all? ie. mkdir -p
	for _, key := range paths {
		_, err := connection.Create(key, nil, 0, zk.WorldACL(zk.PermAll))
		if err != nil && err != zk.ErrNodeExists {
			return err
		}
	}

	return nil
}

// checkExistsFullPath makes sure all the ZNodes
func (ss *ServerSet) checkExistsFullPath(connection *zk.Conn) error {
	paths := splitPaths(ss.directoryPath())

	for _, key := range paths {
		if exists, _, err := connection.Exists(key); !exists || err != nil {
			return err
		}
	}

	return nil
}

// structure of the data in each member znode
// Mimics finagle serverset structure.
type Entity struct {
	ServiceEndpoint     endpoint            `json:"serviceEndpoint"`
	AdditionalEndpoints map[string]endpoint `json:"additionalEndpoints"`
	Shard               int64               `json:"shard"`
	Status              string              `json:"status"`
}

type endpoint struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

func newEntity(host string, port int) *Entity {
	return &Entity{
		ServiceEndpoint:     endpoint{host, port},
		AdditionalEndpoints: make(map[string]endpoint),
		Shard:               0,
		Status:              statusAlive,
	}
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
