package fluent

import (
	"context"
	"time"
)

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

// WithWriteThreshold specifies the minimum number of bytes that we
// should have pending before starting to attempt to write to the
// server. The default value is 8KB
func WithWriteThreshold(i int) Option {
	const name = "write_threshold"
	return &option{
		name:  name,
		value: i,
	}
}

// WithSubsecond specifies if we should use EventTime for timestamps
// on fluentd messages. May be used on a per-client basis or per-call
// to Post(). By default this feature is turned OFF.
//
// Note that this option will only work for fluentd v0.14 or above,
// and you must use gopkg.in/vmihailenco/msgpack.v2 2.9.1 or above.
func WithSubsecond(b bool) Option {
	const name = "subsecond"
	return &option{
		name:  name,
		value: b,
	}
}

// WihContext specifies the context.Context object to be used by Post().
// Possible blocking operations are (1) writing to the background buffer,
// and (2) waiting for a reply from when WithSyncAppend(true) is in use.
func WithContext(ctx context.Context) Option {
	const name = "context"
	return &option{
		name:  name,
		value: ctx,
	}
}

// WithMaxConnAttempts specifies the maximum number of attempts made by
// the client to connect to the fluentd server during final data flushing.
//
// During normal operations, the client will indefinitely attempt to connect
// to the server (whilst being backed-off properly), as it should try as hard
// as possible to send the stored data.
//
// This option controls the behavior when the client still has more data to
// send AFTER it has been told to Close() or Shutdown(). In this case we know
// the client wants to stop at some point, so we try to connect up to a
// finite number of attempts.
//
// The default value is 64.
func WithMaxConnAttempts(n uint64) Option {
	const name = "max_conn_attempts"
	return &option{
		name:  name,
		value: n,
	}
}
