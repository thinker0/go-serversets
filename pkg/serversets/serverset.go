package serversets

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/go-zookeeper/zk"
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

func validatePathElement(name, value string) {
	if strings.Contains(value, "/") {
		panic(fmt.Errorf("%s (%s) must not contain slashes", name, value))
	}
	if value == ".." || strings.Contains(value, "../") || strings.Contains(value, "/..") {
		panic(fmt.Errorf("%s (%s) must not contain directory traversal elements", name, value))
	}
}

// New creates a new ServerSet object that can then be watched
// or have an endpoint added to. The role, environment, and service names must not contain
// slashes or directory traversal elements. Will panic if they do.
func New(role string, environment string, service string, zookeepers []string) *ServerSet {
	validatePathElement("role", role)
	validatePathElement("environment", environment)
	validatePathElement("service", service)

	formatter := NewFinagleFmt(role, environment, service)
	return NewWithFormatter(zookeepers, formatter)
}

// NewWithFormatter creates a new ServerSet with a customized path and data formatter.
func NewWithFormatter(zookeepers []string, zkfmt ZKFmt) *ServerSet {
	ss := &ServerSet{
		ZKTimeout: DefaultZKTimeout,
		ZKFmt:     zkfmt,
		zkServers: zookeepers,
	}

	return ss
}

// NewP is an alias for NewWithFormatter.
func NewP(zookeepers []string, zkfmt ZKFmt) *ServerSet {
	return NewWithFormatter(zookeepers, zkfmt)
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
	fullPath := ss.directoryPath()
	paths := splitPaths(fullPath)

	for _, key := range paths {
		_, err := connection.Create(key, nil, 0, zk.WorldACL(zk.PermAll))
		if err != nil && err != zk.ErrNodeExists {
			return fmt.Errorf("failed to create path %s for node %s: %w", fullPath, key, err)
		}
	}

	return nil
}

// checkExistsFullPath makes sure all the ZNodes exist
func (ss *ServerSet) checkExistsFullPath(connection *zk.Conn) error {
	paths := splitPaths(ss.directoryPath())

	for _, key := range paths {
		exists, _, err := connection.Exists(key)
		if err != nil {
			return fmt.Errorf("failed to check zk node %s existence: %w", key, err)
		}
		if !exists {
			return fmt.Errorf("zk node %s does not exist", key)
		}
	}

	return nil
}
