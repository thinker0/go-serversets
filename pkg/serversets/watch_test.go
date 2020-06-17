package serversets

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"
)

func TestWatchSortEndpoints(t *testing.T) {
	set := New("www", "test", "gotest", []string{TestServer})

	watch, err := set.Watch()
	if err != nil {
		t.Fatal(err)
	}
	defer watch.Close()

	ep1, err := set.RegisterEndpoint("localhost", 1002, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ep1.Close()
	<-watch.Event()

	ep2, err := set.RegisterEndpoint("localhost", 1001, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ep2.Close()
	<-watch.Event()

	ep3, err := set.RegisterEndpoint("localhost", 1003, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ep3.Close()
	<-watch.Event()

	endpoints := watch.Endpoints()
	if len(endpoints) != 3 {
		t.Errorf("should have 3 endpoint, got %v", endpoints)
	}

	if !sort.StringsAreSorted(endpoints) {
		t.Errorf("endpoint list should be sorted, got %v", endpoints)
	}
}

func TestWatchUpdateRecords(t *testing.T) {
	set := New("www", "test", "gotest", []string{TestServer})

	watch, err := set.Watch()
	if err != nil {
		t.Fatal(err)
	}
	defer watch.Close()

	ep1, err := set.RegisterEndpoint("localhost", 1002, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ep1.Close()
	<-watch.Event()

	conn, _, err := set.connectToZookeeper()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	eps, err := watch.updateRecords(conn, []string{set.ZKFmt.Prefix() + "random"})
	if err != nil {
		t.Fatalf("should not have error, got %v", err)
	}

	if len(eps) != 0 {
		t.Errorf("should not have any endpoints, got %v", eps)
	}
}

func TestWatchIsClosed(t *testing.T) {
	set := New("www", "test", "gotest", []string{TestServer})
	watch, err := set.Watch()
	if err != nil {
		t.Fatal(err)
	}

	watch.Close()

	if watch.IsClosed() == false {
		t.Error("should say it's closed right after we close it")
	}
}

func TestWatchMultipleClose(t *testing.T) {
	set := New("www", "test", "gotest", []string{TestServer})
	watch, err := set.Watch()
	if err != nil {
		t.Fatal(err)
	}

	watch.Close()
	watch.Close()
	watch.Close()
}

func TestWatchTriggerEvent(t *testing.T) {
	set := New("www", "test", "gotest", []string{TestServer})
	watch, err := set.Watch()
	if err != nil {
		t.Fatal(err)
	}
	defer watch.Close()

	watch.triggerEvent()
	watch.triggerEvent()
	watch.triggerEvent()
	watch.triggerEvent()
}

type testNode struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	ID      string `json:"id"`
	Port    *int   `json:"port"`
	Enabled bool   `json:"enabled"`
	SSLPort *int   `json:"sslPort"`
}

type testFmt struct{}

func (testFmt) Unmarshal(data []byte) (ZKRecord, error) {
	n := &testNode{}
	err := json.Unmarshal(data, n)
	return n, err
}

func (testFmt) Create(host string, port int) ZKRecord {
	return &testNode{
		Address: host,
		Name:    "test",
		ID:      "id",
		Port:    nil,
		Enabled: true,
		SSLPort: &port,
	}
}

func (testFmt) Path() string   { return "/services/foobar" }
func (testFmt) Prefix() string { return "" }

func (n testNode) Endpoint() string         { return fmt.Sprintf("%s:%d", n.Address, *n.SSLPort) }
func (n testNode) Marshal() ([]byte, error) { return json.Marshal(n) }
func (n testNode) IsAlive() bool            { return n.Enabled }

func TestCustomizedZKFmt(t *testing.T) {
	set := NewP([]string{TestServer}, testFmt{})
	watch, err := set.Watch()
	if err != nil {
		t.Fatal(err)
	}
	defer watch.Close()
	ep, err := set.RegisterEndpoint("localhost", 1002, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ep.Close()
	<-watch.Event()
	expected := "localhost:1002"
	discovered := watch.Endpoints()
	if len(discovered) != 1 {
		t.Fatalf("expected to discover one endpoint, got %v", discovered)
	} else if expected != discovered[0] {
		t.Fatalf("expected to discover %s, got %s", expected, discovered[0])
	}
}
