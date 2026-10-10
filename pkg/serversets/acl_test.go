package serversets

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-zookeeper/zk"
)

func TestACLDefaults(t *testing.T) {
	ss := New("test_role", "test_env", "test_svc", []string{"localhost:2181"})

	expectedDefault := zk.WorldACL(zk.PermAll)
	actualACL := ss.getACL()
	if !reflect.DeepEqual(actualACL, expectedDefault) {
		t.Fatalf("expected default ACL %v, got %v", expectedDefault, actualACL)
	}

	actualMemberACL := ss.getMemberACL()
	if !reflect.DeepEqual(actualMemberACL, expectedDefault) {
		t.Fatalf("expected default MemberACL %v, got %v", expectedDefault, actualMemberACL)
	}
}

func TestSetACLAndDefensiveCopy(t *testing.T) {
	ss := New("test_role", "test_env", "test_svc", []string{"localhost:2181"})

	customACL := []zk.ACL{
		{
			Perms:  zk.PermRead,
			Scheme: "digest",
			ID:     "user:passwdhash",
		},
	}

	ss.SetACL(customACL)

	// Mutate original slice to ensure defensive copy in SetACL
	customACL[0].Perms = zk.PermAll

	got := ss.getACL()
	if len(got) != 1 || got[0].Perms != zk.PermRead {
		t.Fatalf("expected PermRead preserved due to defensive copy, got %v", got)
	}

	// Mutate returned slice to ensure defensive copy in getACL
	got[0].Perms = zk.PermAdmin
	gotAgain := ss.getACL()
	if gotAgain[0].Perms != zk.PermRead {
		t.Fatalf("expected PermRead preserved against mutation of getACL result, got %v", gotAgain)
	}

	// Fallback of MemberACL to ACL
	memberGot := ss.getMemberACL()
	if !reflect.DeepEqual(memberGot, gotAgain) {
		t.Fatalf("expected MemberACL to fall back to ACL, got %v vs %v", memberGot, gotAgain)
	}
}

func TestSetMemberACLAndDefensiveCopy(t *testing.T) {
	ss := New("test_role", "test_env", "test_svc", []string{"localhost:2181"})

	baseACL := []zk.ACL{{Perms: zk.PermRead, Scheme: "world", ID: "anyone"}}
	memberACL := []zk.ACL{{Perms: zk.PermAll, Scheme: "auth", ID: ""}}

	ss.SetACL(baseACL)
	ss.SetMemberACL(memberACL)

	// Mutate original slice
	memberACL[0].Perms = zk.PermWrite

	gotMember := ss.getMemberACL()
	if len(gotMember) != 1 || gotMember[0].Perms != zk.PermAll {
		t.Fatalf("expected MemberACL PermAll preserved, got %v", gotMember)
	}

	gotBase := ss.getACL()
	if len(gotBase) != 1 || gotBase[0].Perms != zk.PermRead {
		t.Fatalf("expected base ACL PermRead, got %v", gotBase)
	}
}

func TestAddAuthAndDefensiveCopy(t *testing.T) {
	ss := New("test_role", "test_env", "test_svc", []string{"localhost:2181"})

	payload := []byte("secret_creds")
	ss.AddAuth("digest", payload)

	// Mutate input byte slice
	payload[0] = 'X'

	if len(ss.Auth) != 1 {
		t.Fatalf("expected 1 auth entry, got %d", len(ss.Auth))
	}
	if string(ss.Auth[0].Payload) != "secret_creds" {
		t.Fatalf("expected payload preserved as 'secret_creds', got %s", string(ss.Auth[0].Payload))
	}
	if ss.Auth[0].Scheme != "digest" {
		t.Fatalf("expected scheme 'digest', got %s", ss.Auth[0].Scheme)
	}

	// Test AuthInfo.Clone
	cloned := ss.Auth[0].Clone()
	cloned.Payload[0] = 'Y'
	if string(ss.Auth[0].Payload) != "secret_creds" {
		t.Fatalf("expected original payload preserved after clone mutation, got %s", string(ss.Auth[0].Payload))
	}
}

func TestAuthInfoMaskingAndZero(t *testing.T) {
	auth := AuthInfo{
		Scheme:  "digest",
		Payload: []byte("super_secret_password"),
	}

	// Test Stringer
	str := auth.String()
	if strings.Contains(str, "super_secret_password") {
		t.Fatalf("String() must redact payload, got: %s", str)
	}
	if !strings.Contains(str, "[REDACTED]") {
		t.Fatalf("String() should contain [REDACTED], got: %s", str)
	}

	// Test GoStringer
	goStr := auth.GoString()
	if strings.Contains(goStr, "super_secret_password") {
		t.Fatalf("GoString() must redact payload, got: %s", goStr)
	}
	if !strings.Contains(goStr, "[REDACTED]") {
		t.Fatalf("GoString() should contain [REDACTED], got: %s", goStr)
	}

	// Test JSON Marshaler
	jsonBytes, err := json.Marshal(auth)
	if err != nil {
		t.Fatalf("failed to marshal AuthInfo to JSON: %v", err)
	}
	if strings.Contains(string(jsonBytes), "super_secret_password") {
		t.Fatalf("JSON output must redact payload, got: %s", string(jsonBytes))
	}
	if !strings.Contains(string(jsonBytes), "[REDACTED]") {
		t.Fatalf("JSON output should contain [REDACTED], got: %s", string(jsonBytes))
	}

	// Test Zero
	auth.Zero()
	if auth.Payload != nil {
		t.Fatalf("Zero() should clear payload to nil, got %v", auth.Payload)
	}
}

func TestMultipleAuthOrder(t *testing.T) {
	ss := New("test_role", "test_env", "test_svc", []string{"localhost:2181"})

	ss.AddAuth("digest", []byte("user1:pass1"))
	ss.AddAuth("digest", []byte("user2:pass2"))
	ss.AddAuth("ip", []byte("10.0.0.1"))

	if len(ss.Auth) != 3 {
		t.Fatalf("expected 3 auth entries, got %d", len(ss.Auth))
	}
	if ss.Auth[0].Scheme != "digest" || string(ss.Auth[0].Payload) != "user1:pass1" {
		t.Errorf("unexpected auth entry 0: %v", ss.Auth[0])
	}
	if ss.Auth[1].Scheme != "digest" || string(ss.Auth[1].Payload) != "user2:pass2" {
		t.Errorf("unexpected auth entry 1: %v", ss.Auth[1])
	}
	if ss.Auth[2].Scheme != "ip" || string(ss.Auth[2].Payload) != "10.0.0.1" {
		t.Errorf("unexpected auth entry 2: %v", ss.Auth[2])
	}
}

func TestOptionsAndApply(t *testing.T) {
	customACL := []zk.ACL{{Perms: zk.PermRead, Scheme: "world", ID: "anyone"}}
	memberACL := []zk.ACL{{Perms: zk.PermAll, Scheme: "auth", ID: ""}}

	ss := New("test_role", "test_env", "test_svc", []string{"localhost:2181"}).
		Apply(
			WithACL(customACL),
			WithMemberACL(memberACL),
			WithAuth("digest", []byte("user:pass")),
			WithZKTimeout(10*time.Second),
			WithCreatePathOnWatch(true),
		)

	if !reflect.DeepEqual(ss.getACL(), customACL) {
		t.Fatalf("WithACL not applied correctly: got %v", ss.getACL())
	}
	if !reflect.DeepEqual(ss.getMemberACL(), memberACL) {
		t.Fatalf("WithMemberACL not applied correctly: got %v", ss.getMemberACL())
	}
	if len(ss.Auth) != 1 || ss.Auth[0].Scheme != "digest" || string(ss.Auth[0].Payload) != "user:pass" {
		t.Fatalf("WithAuth not applied correctly: got %v", ss.Auth)
	}
	if ss.ZKTimeout != 10*time.Second {
		t.Fatalf("WithZKTimeout not applied correctly: got %v", ss.ZKTimeout)
	}
	if !ss.CreatePathOnWatch {
		t.Fatalf("WithCreatePathOnWatch not applied correctly: got %v", ss.CreatePathOnWatch)
	}
}

func TestNilResetAndEmptySliceFallback(t *testing.T) {
	ss := New("test_role", "test_env", "test_svc", []string{"localhost:2181"})

	ss.SetACL([]zk.ACL{{Perms: zk.PermRead, Scheme: "world", ID: "anyone"}})
	ss.SetMemberACL([]zk.ACL{{Perms: zk.PermWrite, Scheme: "auth", ID: ""}})

	// Reset with nil
	ss.SetACL(nil)
	ss.SetMemberACL(nil)

	if !reflect.DeepEqual(ss.getACL(), zk.WorldACL(zk.PermAll)) {
		t.Fatalf("expected fallback to WorldACL(PermAll) after SetACL(nil)")
	}
	if !reflect.DeepEqual(ss.getMemberACL(), zk.WorldACL(zk.PermAll)) {
		t.Fatalf("expected fallback to WorldACL(PermAll) after SetMemberACL(nil)")
	}

	// Reset with empty slice []zk.ACL{}
	ss.SetACL([]zk.ACL{})
	ss.SetMemberACL([]zk.ACL{})

	if !reflect.DeepEqual(ss.getACL(), zk.WorldACL(zk.PermAll)) {
		t.Fatalf("expected fallback to WorldACL(PermAll) after SetACL([])")
	}
	if !reflect.DeepEqual(ss.getMemberACL(), zk.WorldACL(zk.PermAll)) {
		t.Fatalf("expected fallback to WorldACL(PermAll) after SetMemberACL([])")
	}
}

func TestConnectToZookeeperTimeoutFallback(t *testing.T) {
	ss := New("test_role", "test_env", "test_svc", []string{"127.0.0.1:65530"})
	ss.ZKTimeout = 0 // Invalid/zero timeout

	// connectToZookeeper should fall back to DefaultZKTimeout without panic
	conn, _, err := ss.connectToZookeeper()
	if err == nil && conn != nil {
		conn.Close()
	}
}

func TestZKACLWithRealServer(t *testing.T) {
	requireZK(t)

	// If ZooKeeper is available, verify creating nodes with custom ACL works
	set := New("test", "test", "acltest", []string{TestServer}).
		Apply(
			WithACL(zk.WorldACL(zk.PermRead|zk.PermCreate|zk.PermAdmin)),
			WithMemberACL(zk.WorldACL(zk.PermAll)),
		)

	ep, err := set.RegisterEndpoint("localhost", 9999, nil)
	if err != nil {
		t.Fatalf("registration failure with ACL: %v", err)
	}
	defer ep.Close()

	watch, err := set.Watch()
	if err != nil {
		t.Fatalf("watch failure with ACL: %v", err)
	}
	defer watch.Close()

	<-watch.Event()
	if !reflect.DeepEqual(watch.Endpoints(), []string{"localhost:9999"}) {
		t.Errorf("expected localhost:9999, got %v", watch.Endpoints())
	}
}
