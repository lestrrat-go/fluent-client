package fluent

import (
	"sync"
	"time"
)

// Client represents a fluentd client. The client receives data as we go,
// and proxies it to a background writer. The background writer attempts to
// write to the server as soon as possible
type Client struct {
	address      string // network address (host:port) or socket path
	bufferSize   int
	bufferLimit  int
	dialTimeout  time.Duration // max time to wait when connecting
	marshaler    Marshaler
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

type Marshaler interface {
	Marshal(string, int64, interface{}) ([]byte, error)
}

type JSONMarshaler struct{}
type MsgpackMarshaler struct{}
