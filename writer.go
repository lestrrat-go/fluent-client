package fluent

import (
	"context"
	"log"
	"net"
	"sync"
	"time"
)

// Starts the background writer.
func (c *Client) startWriter() {
	c.muWriter.Lock()
	defer c.muWriter.Unlock()
	if c.writerCancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	c.writerCancel = cancel
	writer := newWriter(c)

	go writer.runReader(ctx)
	go writer.runWriter(ctx)
}

type writer struct {
	address       string
	cond          *sync.Cond
	dialTimeout   time.Duration
	done          chan struct{}
	incoming      chan []byte
	muPending     sync.Mutex
	network       string
	pending       []byte
	updateBufsize func(int)
	writeTimeout  time.Duration
}

func newWriter(c *Client) *writer {
	var w writer

	w.address = c.address
	w.cond = sync.NewCond(&sync.Mutex{})
	w.dialTimeout = c.dialTimeout
	w.done = make(chan struct{})
	// unbuffered, so the writer knows that immediately upon
	// write success, we received it
	w.incoming = make(chan []byte)
	w.network = c.network
	w.pending = make([]byte, 0, c.bufferLimit)
	w.updateBufsize = c.updateBufsize
	w.writeTimeout = 3 * time.Second

	// Copy relevant data to client
	c.writerQueue = w.incoming
	c.writerExit = w.done

	return &w
}

func (w *writer) runReader(ctx context.Context) {
	// This goroutine receives the incoming data as fast as
	// possible, so that the caller to enqueue does not block
	for {
		select {
		case <-ctx.Done():
			// Wake up the writer goroutine so that it can detect
			// cancelation
			w.cond.Broadcast()
			return
		case data := <-w.incoming:
			w.muPending.Lock()
			w.pending = append(w.pending, data...)
			w.updateBufsize(len(w.pending))
			w.muPending.Unlock()

			// Wake up the writer goroutine
			w.cond.Broadcast()
		}
	}
}

func (w *writer) runWriter(ctx context.Context) {
	defer close(w.done)

	// This goroutine waits for the receiver goroutine to wake
	// it up. When it's awake, we know that there's at least one
	// piece of data to send to the fluentd server.
	var conn net.Conn
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()
	for {
		// We need to check for ctx.Done() here before getting into
		// the cond loop, because otherwise we might never be woken
		// up again
		select {
		case <-ctx.Done():
			return
		default:
		}

		w.cond.L.Lock()
		for len(w.pending) == 0 {
			select {
			case <-ctx.Done():
				return
			default:
			}
			w.cond.Wait()
		}
		w.cond.L.Unlock()

		// if we're not connected, we should do that now.
		if conn == nil {
			var dialer net.Dialer
			connCtx, cancel := context.WithTimeout(ctx, w.dialTimeout)
			for conn == nil {
				select {
				case <-connCtx.Done():
					cancel()
					return
				default:
					conn, _ = dialer.DialContext(connCtx, w.network, w.address)
				}
			}
			cancel()
		}

		conn.SetWriteDeadline(time.Now().Add(w.writeTimeout))

		w.muPending.Lock()
		_, err := conn.Write(w.pending)
		if err != nil {
			conn.Close()
			conn = nil
		} else {
			w.updateBufsize(0)
			w.pending = w.pending[:0]
		}
		w.muPending.Unlock()
	}
}
