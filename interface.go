package fluent

import (
	"sync"
	"time"
)

type marshaler interface {
	Marshal(*Message) ([]byte, error)
}

// Client represents a fluentd client. The client receives data as we go,
// and proxies it to a background minion. The background minion attempts to
// write to the server as soon as possible
type Client struct {
	closed       bool
	minionCancel func()
	minionDone   chan struct{}
	minionQueue  chan *Message
	muClosed     sync.RWMutex
	subsecond    bool
}

// Option is an interface used for providing options to the
// various methods
type Option interface {
	Name() string
	Value() interface{}
}

// Message is a fluentd's payload, which can be encoded in JSON or MessagePack
// format.
type Message struct {
	Tag       string      `msgpack:"tag"`
	Time      EventTime   `msgpack:"time"`
	Record    interface{} `msgpack:"record"`
	Option    interface{} `msgpack:"option"`
	subsecond bool        // true if we should include subsecond resolution time
	replyCh   chan error  // non-nil if caller expects notification for successfully appending to buffer
}

type EventTime struct {
	time.Time
}
