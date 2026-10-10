package serversets

import (
	"net"
	"testing"
	"time"
)

func requireZK(t *testing.T) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", TestServer+":2181", 500*time.Millisecond)
	if err != nil {
		t.Skipf("skipping test because ZooKeeper is not reachable at %s:2181: %v", TestServer, err)
	}
	_ = conn.Close()
}
