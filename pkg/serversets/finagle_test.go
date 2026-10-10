package serversets

import (
	"testing"
)

func TestFinagleFmtCreateAndMarshal(t *testing.T) {
	fmt := NewFinagleFmt("testrole", "testenv", "testsvc")

	if fmt.Path() != "/aurora/testrole/testenv/testsvc" {
		t.Errorf("unexpected path: %s", fmt.Path())
	}
	if fmt.Prefix() != "member_" {
		t.Errorf("unexpected prefix: %s", fmt.Prefix())
	}

	record := fmt.Create("127.0.0.1", 8080)
	if !record.IsAlive() {
		t.Errorf("expected record to be alive")
	}
	if record.Endpoint() != "127.0.0.1:8080" {
		t.Errorf("unexpected endpoint: %s", record.Endpoint())
	}

	bytes, err := record.Marshal()
	if err != nil {
		t.Fatalf("failed to marshal record: %v", err)
	}

	decoded, err := fmt.Unmarshal(bytes)
	if err != nil {
		t.Fatalf("failed to unmarshal record: %v", err)
	}

	if decoded.Endpoint() != "127.0.0.1:8080" {
		t.Errorf("unexpected endpoint after unmarshal: %s", decoded.Endpoint())
	}
	if !decoded.IsAlive() {
		t.Errorf("expected decoded record to be alive")
	}
}
