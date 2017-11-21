package fluent

import (
	"context"
	"io"
	"net"
	"time"

	pdebug "github.com/lestrrat/go-pdebug"
	"github.com/pkg/errors"
)

// NewUnbuffered creates an unbuffered client. Unlike the normal
// buffered client, an unbuffered client handles the Post() method
// synchronously, and does not attempt to buffer the payload.
//
//    * fluent.WithAddress
//    * fluent.WithDialTimeout
//    * fluent.WithMarshaler
//    * fluent.WithMaxConnAttempts
//    * fluent.WithNetwork
//    * fluent.WithSubSecond
//    * fluent.WithTagPrefix
//
// Please see their respective documentation for details.
func NewUnbuffered(options ...Option) (*Unbuffered, error) {
	var c = &Unbuffered{
		address:         "127.0.0.1:24224",
		dialTimeout:     3 * time.Second,
		maxConnAttempts: 64,
		marshaler:       marshalFunc(msgpackMarshal),
		network:         "tcp",
		writeTimeout:    3 * time.Second,
	}

	for _, opt := range options {
		switch opt.Name() {
		case optkeyAddress:
			c.address = opt.Value().(string)
		case optkeyDialTimeout:
			c.dialTimeout = opt.Value().(time.Duration)
		case optkeyMarshaler:
			c.marshaler = opt.Value().(marshaler)
		case optkeyMaxConnAttempts:
			c.maxConnAttempts = opt.Value().(uint64)
		case optkeyNetwork:
			v := opt.Value().(string)
			switch v {
			case "tcp", "unix":
			default:
				return nil, errors.Errorf(`invalid network type: %s`, v)
			}
			c.network = v
		case optkeySubSecond:
			c.subsecond = opt.Value().(bool)
		case optkeyTagPrefix:
			c.tagPrefix = opt.Value().(string)
		}
	}

	return c, nil
}

// Close cloes the currenct cached connection, if any
func (c *Unbuffered) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}
	c.conn.Close()
	c.conn = nil
	return nil
}

// Shutdown is an alias to Close(). Since an unbuffered
// Client does not have any pending buffers at any given moment,
// we do not have to do anything other than close
func (c *Unbuffered) Shutdown(_ context.Context) error {
	return c.Close()
}

func (c *Unbuffered) connect(force bool) (net.Conn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		if !force {
			return c.conn, nil
		}
		c.conn.Close()
	}

	connCtx, cancel := context.WithTimeout(context.Background(), c.dialTimeout)
	defer cancel()

	var dialer net.Dialer
	conn, err := dialer.DialContext(connCtx, c.network, c.address)
	if err != nil {
		return nil, errors.Wrap(err, `failed to connect to server`)
	}

	c.conn = conn
	return conn, nil
}

// Post posts the given structure after encoding it along with the given tag.
//
// If you would like to specify options to `Post()`, you may pass them at the
// end of the method. Currently you can use the following:
//
//   fluent.WithTimestamp: allows you to set arbitrary timestamp values
//
func (c *Unbuffered) Post(tag string, v interface{}, options ...Option) (err error) {
	if pdebug.Enabled {
		g := pdebug.Marker("fluent.Unbuffered.Post").BindError(&err)
		defer g.End()
	}

	var t time.Time
	for _, opt := range options {
		switch opt.Name() {
		case optkeyTimestamp:
			t = opt.Value().(time.Time)
		}
	}

	if t.IsZero() {
		t = time.Now()
	}
	msg := getMessage()
	msg.Tag = tag
	msg.Time.Time = t
	msg.Record = v
	msg.subsecond = c.subsecond
	defer releaseMessage(msg)

	serialized, err := c.marshaler.Marshal(msg)
	if err != nil {
		return errors.Wrap(err, `failed to serialize payload`)
	}

	var attempt uint64
WRITE:
	attempt++
	if pdebug.Enabled {
		pdebug.Printf("Attempt %d/%d", attempt, c.maxConnAttempts)
	}
	payload := serialized
	if attempt > c.maxConnAttempts {
		return errors.New(`exceeded max connection attempts`)
	}

	conn, err := c.connect(attempt > 1)
	if err != nil {
		goto WRITE
	}
	if pdebug.Enabled {
		pdebug.Printf("Successfully connected to server")
	}

	if pdebug.Enabled {
		pdebug.Printf("Going to write %d bytes", len(payload))
	}

	for len(payload) > 0 {
		n, err := conn.Write(payload)
		if err != nil {
			if err == io.EOF {
				goto WRITE // Try again
			}

			return errors.Wrap(err, `failed to write serialized payload`)
		}
		if pdebug.Enabled {
			pdebug.Printf("Wrote %d bytes", n)
		}
		payload = payload[n:]
	}

	// All done!
	return nil
}
