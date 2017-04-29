// Package fluent implements a client for the fluentd data logging daemon.
package fluent

import (
	"context"
	"time"

	pdebug "github.com/lestrrat/go-pdebug"
	"github.com/pkg/errors"
)

// New creates a new client.
func New(options ...Option) (*Client, error) {
	c := &Client{
		address:     "127.0.0.1:24224",
		bufferLimit: 8 * 1024 * 1024,
		dialTimeout: 3 * time.Second,
		marshaler:   marshalFunc(msgpackMarshal),
		minionDone:  make(chan struct{}),
		network:     "tcp",
	}

	for _, opt := range options {
		switch opt.Name() {
		case "network":
			v := opt.Value().(string)
			switch v {
			case "tcp", "unix":
			default:
				return nil, errors.Errorf(`invalid network type: %s`, v)
			}
			c.network = v
		case "address":
			c.address = opt.Value().(string)
		case "buffer_limit":
			c.bufferLimit = opt.Value().(int)
		case "dialTimeout":
			c.dialTimeout = opt.Value().(time.Duration)
		case "marshaler":
			c.marshaler = opt.Value().(marshaler)
		case "tag_prefix":
			c.tagPrefix = opt.Value().(string)
		}
	}

	c.startMinion()
	return c, nil
}

// Post posts the given structure after encoding it along with the given tag.
// An error is returned if the background writer goroutine is not currently
// running.
//
// If you would like to specify options, you may pass them at the end of
// the method. Currently you can use the following:
//
// fluent.WithTimestamp: allows you to set arbitrary timestamp values
// fluent.WithSyncAppend: allows you to verify if the append was successful
//
// If fluent.WithSyncAppend is provide and is true, the following errors
// may be returned:
//
// 1. If the current underlying pending buffer is checked for its size. If it
// is not large enough to hold this new data, an error will be returned
// 2. If the marshaling into msgpack/json failed, it is returned
//
func (c *Client) Post(tag string, v interface{}, options ...Option) error {
	if p := c.tagPrefix; len(p) > 0 {
		tag = p + "." + tag
	}

	var syncAppend bool
	var t int64
	for _, opt := range options {
		switch opt.Name() {
		case "timestamp":
			t = opt.Value().(time.Time).Unix()
		case "sync_append":
			syncAppend = opt.Value().(bool)
		}
	}
	if t == 0 {
		t = time.Now().Unix()
	}

	msg := getMessage()
	msg.Tag = tag
	msg.Time = t
	msg.Record = v

	// This has to be separate from msg.replyCh, b/c msg would be
	// put back to the pool
	var replyCh chan error
	if syncAppend {
		replyCh = make(chan error)
		msg.replyCh = replyCh
	}

	select {
	case <-c.minionDone:
		return errors.New("writer has been closed. Shutdown called?")
	case c.minionQueue <- msg:
	}

	if syncAppend {
		if pdebug.Enabled {
			pdebug.Printf("client: Post is waiting for return status")
		}
		select {
		case <-c.minionDone:
			return errors.New("writer has been closed. Shutdown called?")
		case e := <-replyCh:
			if pdebug.Enabled {
				pdebug.Printf("client: synchronous result received")
			}
			return e
		}
	}

	return nil
}

// Close closes the connection, but does not wait for the pending buffers
// to be flushed. If you want to make sure that background minion has properly
// exited, you should probably use the Shutdown() method
func (c *Client) Close() error {
	c.minionCancel()
	return nil
}

// Shutdown closes the connection. This method will block until the
// background minion exits, or the caller explicitly cancels the
// provided context object.
func (c *Client) Shutdown(ctx context.Context) error {
	if pdebug.Enabled {
		pdebug.Printf("client: shutdown requested")
		defer pdebug.Printf("client: shutdown completed")
	}

	if ctx == nil {
		ctx = context.Background() // no cancel...
	}

	if err := c.Close(); err != nil {
		return errors.Wrap(err, `failed to close`)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.minionDone:
		return nil
	}
}
