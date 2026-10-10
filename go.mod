module github.com/thinker0/go-serversets

go 1.21

// Retract broken tags that had mismatched module path (github.com/zmstone/go-serversets)
retract (
	v1.3.1 // Declared module path github.com/zmstone/go-serversets
	v1.3.0 // Declared module path github.com/zmstone/go-serversets
)

require (
	github.com/apache/thrift v0.13.0
	github.com/go-zookeeper/zk v1.0.2
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e
	github.com/reusee/mmh3 v0.0.0-20140820141314-64b85163255b
)
