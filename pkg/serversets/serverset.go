package serversets

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/samuel/go-zookeeper/zk"
)

// DefaultZKTimeout is the zookeeper timeout used if it is not overwritten.
var DefaultZKTimeout = 5 * time.Second

// A ServerSet represents a service with a set of servers that may change over time.
// The master lists of servers is kept as ephemeral nodes in Zookeeper.
type ServerSet struct {
	ZKTimeout time.Duration
	ZKFmt     ZKFmt
	zkServers []string
}

// New creates a new ServerSet object that can then be watched
// or have an endpoint added to. The service name must not contain
// any slashes. Will panic if it does.
func New(role string, environment string, service string, zookeepers []string) *ServerSet {
	if strings.Contains(service, "/") {
		panic(fmt.Errorf("service name (%s) must not contain slashes", service))
	}

	fmt := NewFinagleFmt(role, environment, service)
	return NewP(zookeepers, fmt)
}

// NewP creates a new ServerSet with customized path/data formatter.
func NewP(zookeepers []string, zkfmt ZKFmt) *ServerSet {
	ss := &ServerSet{
		ZKTimeout: DefaultZKTimeout,
		ZKFmt:     zkfmt,
		zkServers: zookeepers,
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
	return ss.ZKFmt.Path()
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
	path := ss.directoryPath()
	paths := splitPaths(path)

	// TODO: can't we just create all? ie. mkdir -p
	for _, key := range paths {
		_, err := connection.Create(key, nil, 0, zk.WorldACL(zk.PermAll))
		if err != nil && err != zk.ErrNodeExists {
			return fmt.Errorf("failed to create %s for node %s, %v", path, key, err)
		}
	}

	return nil
}

// checkExistsFullPath makes sure all the ZNodes
func (ss *ServerSet) checkExistsFullPath(connection *zk.Conn) error {
	paths := splitPaths(ss.directoryPath())

	for _, key := range paths {
		exists, _, err := connection.Exists(key)
		if !exists {
			return fmt.Errorf("zk node %s does not exist", key)
		}
		if err != nil {
			return fmt.Errorf("failed to check zk node %s existence, %v", key, err)
		}
	}

	return nil
}
