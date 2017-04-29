package fluent

import "time"

type option struct {
	name  string
	value interface{}
}

func (o *option) Name() string {
	return o.name
}

func (o *option) Value() interface{} {
	return o.value
}

// WithNetwork specifies the network type, i.e. "tcp" or "unix"
// for `fluent.New`
func WithNetwork(s string) Option {
	const name = "network"
	return &option{
		name:  name,
		value: s,
	}
}

// WithAddress specifies the address to connect to for `fluent.New`
// A unix domain socket path, or a hostname/IP address.
func WithAddress(s string) Option {
	const name = "address"
	return &option{
		name:  name,
		value: s,
	}
}

// WithTimestamp specifies the timestamp to be used for `Client.Post`
func WithTimestamp(t time.Time) Option {
	const name = "timestamp"
	return &option{
		name:  name,
		value: t,
	}
}

// WithJSONMarshaler specifies JSON marshaling to be used when
// sending messages to fluentd. Used for `fluent.New`
func WithJSONMarshaler() Option {
	const name = "marshaler"
	return &option{
		name:  name,
		value: marshalFunc(jsonMarshal),
	}
}

// WithMsgpackMarshaler specifies msgpack marshaling to be used when
// sending messages to fluentd. Used in `fluent.New`
func WithMsgpackMarshaler() Option {
	const name = "marshaler"
	return &option{
		name:  name,
		value: marshalFunc(msgpackMarshal),
	}
}

// WithTagPrefix specifies the prefix to be appended to tag names
// when sending messages to fluend. Used in `fluent.New`
func WithTagPrefix(s string) Option {
	const name = "tag_prefix"
	return &option{
		name:  name,
		value: s,
	}
}

// WithSyncAppend specifies if we should synchronously check for
// success when appending to the underlying pending buffer.
// Used in `Client.Post`. If not specified, errors appending
// are not reported.
func WithSyncAppend(b bool) Option {
	const name = "sync_append"
	return &option{
		name:  name,
		value: b,
	}
}

// WithBufferLimit specifies the buffer limit to be used for
// the underlying pending buffer. If a `Client.Post` operation
// would exceed this size, an error is returned (note: you must
// use `WithSyncAppend` in `Client.Post` if you want this error
// to be reported)
func WithBufferLimit(v interface{}) Option {
	const name = "buffer_limit"
	return &option{
		name:  name,
		value: v,
	}
}
