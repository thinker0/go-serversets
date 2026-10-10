package serversets

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-zookeeper/zk"
)

// AuthInfo represents authentication credentials for ZooKeeper.
// Sensitive credential payload is automatically redacted in fmt and JSON outputs.
type AuthInfo struct {
	Scheme  string
	Payload []byte
}

// Clone returns a deep copy of the AuthInfo.
func (a AuthInfo) Clone() AuthInfo {
	payload := make([]byte, len(a.Payload))
	copy(payload, a.Payload)
	return AuthInfo{
		Scheme:  a.Scheme,
		Payload: payload,
	}
}

// String implements fmt.Stringer, redacting the sensitive payload.
func (a AuthInfo) String() string {
	return fmt.Sprintf("AuthInfo{Scheme: %s, Payload: [REDACTED]}", a.Scheme)
}

// GoString implements fmt.GoStringer, redacting the sensitive payload.
func (a AuthInfo) GoString() string {
	return fmt.Sprintf("serversets.AuthInfo{Scheme: %q, Payload: []byte(\"[REDACTED]\")}", a.Scheme)
}

// MarshalJSON implements json.Marshaler, preventing credential leakage in JSON logs.
func (a AuthInfo) MarshalJSON() ([]byte, error) {
	type authInfoMasked struct {
		Scheme  string `json:"scheme"`
		Payload string `json:"payload"`
	}
	return json.Marshal(authInfoMasked{
		Scheme:  a.Scheme,
		Payload: "[REDACTED]",
	})
}

// Zero wipes the credential payload bytes in memory.
func (a *AuthInfo) Zero() {
	for i := range a.Payload {
		a.Payload[i] = 0
	}
	a.Payload = nil
}

// getACL returns the base ACL configured for the ServerSet.
// If ss.ACL is nil or empty, it defaults to zk.WorldACL(zk.PermAll) for backward compatibility.
func (ss *ServerSet) getACL() []zk.ACL {
	if len(ss.ACL) > 0 {
		copied := make([]zk.ACL, len(ss.ACL))
		copy(copied, ss.ACL)
		return copied
	}
	return zk.WorldACL(zk.PermAll)
}

// getMemberACL returns the ACL for ephemeral member znodes.
// If MemberACL is configured, it is returned; otherwise it falls back to getACL().
func (ss *ServerSet) getMemberACL() []zk.ACL {
	if len(ss.MemberACL) > 0 {
		copied := make([]zk.ACL, len(ss.MemberACL))
		copy(copied, ss.MemberACL)
		return copied
	}
	return ss.getACL()
}

// AddAuth adds an authentication credential to the ServerSet.
// Must be called during initialization before Watch() or RegisterEndpoint().
func (ss *ServerSet) AddAuth(scheme string, auth []byte) {
	payload := make([]byte, len(auth))
	copy(payload, auth)
	ss.Auth = append(ss.Auth, AuthInfo{
		Scheme:  scheme,
		Payload: payload,
	})
}

// SetACL sets the base ACL for creating parent directories.
// Must be called during initialization before Watch() or RegisterEndpoint().
// Passing nil or an empty slice resets to the default zk.WorldACL(zk.PermAll).
func (ss *ServerSet) SetACL(acl []zk.ACL) {
	if len(acl) == 0 {
		ss.ACL = nil
		return
	}
	copied := make([]zk.ACL, len(acl))
	copy(copied, acl)
	ss.ACL = copied
}

// SetMemberACL sets the ACL for ephemeral member znodes.
// Must be called during initialization before Watch() or RegisterEndpoint().
// Passing nil or an empty slice resets to fallback to the base ACL.
func (ss *ServerSet) SetMemberACL(acl []zk.ACL) {
	if len(acl) == 0 {
		ss.MemberACL = nil
		return
	}
	copied := make([]zk.ACL, len(acl))
	copy(copied, acl)
	ss.MemberACL = copied
}

// Option is a configuration function for ServerSet.
type Option func(*ServerSet)

// WithACL sets the base ACL for parent directories.
func WithACL(acl []zk.ACL) Option {
	return func(ss *ServerSet) {
		ss.SetACL(acl)
	}
}

// WithMemberACL sets the ACL for ephemeral member nodes.
func WithMemberACL(acl []zk.ACL) Option {
	return func(ss *ServerSet) {
		ss.SetMemberACL(acl)
	}
}

// WithAuth adds authentication credentials to the ServerSet.
func WithAuth(scheme string, auth []byte) Option {
	return func(ss *ServerSet) {
		ss.AddAuth(scheme, auth)
	}
}

// WithZKTimeout sets the ZooKeeper session timeout.
func WithZKTimeout(timeout time.Duration) Option {
	return func(ss *ServerSet) {
		ss.ZKTimeout = timeout
	}
}

// WithCreatePathOnWatch configures whether Watch() should ensure the parent znode path exists.
func WithCreatePathOnWatch(create bool) Option {
	return func(ss *ServerSet) {
		ss.CreatePathOnWatch = create
	}
}

// Apply applies configuration options to the ServerSet.
// Must be called during initialization before Watch() or RegisterEndpoint().
func (ss *ServerSet) Apply(options ...Option) *ServerSet {
	for _, opt := range options {
		opt(ss)
	}
	return ss
}
