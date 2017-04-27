package fluent

import (
	"context"
	"time"

	"github.com/pkg/errors"
)

func New(options ...Option) (*Client, error) {
	c := &Client{
		address:     "127.0.0.1:24224",
		bufferLimit: 8 * 1024 * 1024,
		dialTimeout: 3 * time.Second,
		marshaler:   &MsgpackMarshaler{},
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
		case "dialTimeout":
			c.dialTimeout = opt.Value().(time.Duration)
		case "marshaler":
			c.marshaler = opt.Value().(Marshaler)
		case "tag_prefix":
			c.tagPrefix = opt.Value().(string)
		}
	}

	return c, nil
}

// called from the writer process
func (c *Client) updateBufsize(v int) {
	c.muBufsize.Lock()
	c.bufferSize = v
	c.muBufsize.Unlock()
}

// Post posts the given structure after encoding it along with the given
// tag. If the current underlying pending buffer is not enough to hold
// this new data, an error will be returned
func (c *Client) Post(tag string, v interface{}, options ...Option) error {
	c.startWriter()

	if p := c.tagPrefix; len(p) > 0 {
		tag = p + "." + tag
	}
		
	t := time.Now()
	for _, opt := range options {
		switch opt.Name() {
		case "timestamp":
			t = opt.Value().(time.Time)
		}
	}
	buf, err := c.marshaler.Marshal(tag, t.Unix(), v)
	if err != nil {
		return errors.Wrap(err, `failed to marshal payload`)
	}

	c.muBufsize.RLock()
	isFull := c.bufferSize+len(buf) > c.bufferLimit
	c.muBufsize.RUnlock()

	if isFull {
		return errors.New("buffer full")
	}

	c.muWriter.Lock()
	if c.writerCancel == nil {
		c.muWriter.Unlock()
		return errors.New("writer has been closed. Shutdown called?")
	}
	c.writerQueue <- buf
	c.muWriter.Unlock()

	return nil
}

// Shutdown closes the connection. This method will block until the
// background writer exits, or the caller explicitly cancels the
// provided context object.
func (c *Client) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background() // no cancel...
	}

	c.muWriter.Lock()
	if c.writerCancel == nil {
		c.muWriter.Unlock()
		return nil
	}

	// fire the cancel function. the background writer should
	// attempt to flush all of its pending buffers
	c.writerCancel()
	c.writerCancel = nil
	writerDone := c.writerExit
	c.muWriter.Unlock()

	select {
	case <-ctx.Done():
	case <-writerDone:
		return nil
	}
	return ctx.Err()
}
