package fluent

import (
	"context"
	"net"
	"sync"
	"time"
)

type marshaler interface {
	Marshal(*Message) ([]byte, error)
}

// Client represents a fluentd client. The client receives data as we go,
// and proxies it to a background minion. The background minion attempts to
// write to the server as soon as possible
type Client interface {
	Post(string, interface{}, ...Option) error
	Ping(string, interface{}, ...Option) error
	Close() error
	Shutdown(context.Context) error
}

// Buffered is a Client that buffers incoming messages, and sends them
// asynchrnously when it can.
//
//nolint:maligned
type Buffered struct {
	closed       bool
	minionCancel func()
	minionDone   chan struct{}
	minionQueue  chan *Message
	muClosed     sync.RWMutex
	pingQueue    chan *Message
	subsecond    bool
}

// Unbuffered is a Client that synchronously sends messages.
type Unbuffered struct {
	address         string
	conn            net.Conn
	dialTimeout     time.Duration
	marshaler       marshaler
	maxConnAttempts uint64
	mu              sync.RWMutex
	network         string
	subsecond       bool
	tagPrefix       string
	writeTimeout    time.Duration
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

// EventTime is used to represent the time in a msgpack Message
type EventTime struct {
	time.Time
}
