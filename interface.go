package fluent

type marshaler interface {
	Marshal(*Message) ([]byte, error)
}

// Client represents a fluentd client. The client receives data as we go,
// and proxies it to a background minion. The background minion attempts to
// write to the server as soon as possible
type Client struct {
	minionCancel func()
	minionDone   chan struct{}
	minionQueue  chan *Message
}

type Option interface {
	Name() string
	Value() interface{}
}

// Message is a fluentd's payload, which can be encoded in JSON or MessagePack
// format.
type Message struct {
	Tag     string      `msgpack:"tag"`
	Time    int64       `msgpack:"time"`
	Record  interface{} `msgpack:"record"`
	Option  interface{} `msgpack:"option"`
	replyCh chan error
}
