package fluent

import (
	"sync"
	"time"
)

type marshaler interface {
	Marshal(string, int64, interface{}, interface{}) ([]byte, error)
}

// Client represents a fluentd client. The client receives data as we go,
// and proxies it to a background writer. The background writer attempts to
// write to the server as soon as possible
type Client struct {
	address      string // network address (host:port) or socket path
	bufferSize   int
	bufferLimit  int
	dialTimeout  time.Duration // max time to wait when connecting
	marshaler    marshaler
	muBufsize    sync.RWMutex
	muWriter     sync.Mutex
	network      string // tcp or unix
	tagPrefix    string
	writerCancel func()
	writerExit   chan struct{}
	writerQueue  chan []byte
}

type Option interface {
	Name() string
	Value() interface{}
}

// Message is a fluentd's payload, which can be encoded in JSON or MessagePack
// format.
type Message struct {
	Tag    string      `msgpack:"tag"`
	Time   int64       `msgpack:"time"`
	Record interface{} `msgpack:"record"`
	Option interface{} `msgpack:"option"`
}
