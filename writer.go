package fluent

import (
	"context"
	"net"
	"sync"
	"time"

	pdebug "github.com/lestrrat/go-pdebug"
)

// Architecture:
//
// The Client passes encoded bytes to a channel where the minion reader
// is reading from. The minion reader goroutine is responsible for accepting
// these encoded bytes from the Client as soon as possible, as the Client
// is being blocked while this is happening. The minion reader appends the
// new bytes to a "pending" byte slice, and immediately goes back to waiting
// for new bytes coming in from the client.
//
// Meanwhile, a minion writer is woken up by the reader via a sync.Cond.
// The minion writer checks to see if there are any pending bytes to write
// to the server. If there's anything, we start the write process
//

// Starts the background writer.
func (c *Client) startMinion() {
	c.muMinion.Lock()
	defer c.muMinion.Unlock()
	if c.minionCancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	c.minionCancel = cancel
	m := newMinion(c)

	go m.runReader(ctx)
	go m.runWriter(ctx)
}

type minion struct {
	address       string
	cond          *sync.Cond
	dialTimeout   time.Duration
	done          chan struct{}
	flush         bool
	incoming      chan []byte
	muFlush       sync.RWMutex
	muPending     sync.RWMutex
	network       string
	pending       []byte
	updateBufsize func(int)
	writeTimeout  time.Duration
}

func newMinion(c *Client) *minion {
	var m minion

	m.address = c.address
	m.cond = sync.NewCond(&sync.Mutex{})
	m.dialTimeout = c.dialTimeout
	m.done = make(chan struct{})
	// unbuffered, so the writer knows that immediately upon
	// write success, we received it
	m.incoming = make(chan []byte)
	m.network = c.network
	m.pending = make([]byte, 0, c.bufferLimit)
	m.updateBufsize = c.updateBufsize
	m.writeTimeout = 3 * time.Second

	// Copy relevant data to client
	c.minionQueue = m.incoming
	c.minionExit = m.done

	return &m
}

func (m *minion) runReader(ctx context.Context) {
	if pdebug.Enabled {
		pdebug.Printf("background reader: starting")
		defer pdebug.Printf("background reader: exiting")
	}
	// This goroutine receives the incoming data as fast as
	// possible, so that the caller to enqueue does not block
	for {
		select {
		case <-ctx.Done():
			// Wake up the writer goroutine so that it can detect
			// cancelation
			m.muFlush.Lock()
			m.flush = true
			m.muFlush.Unlock()

			m.cond.Broadcast()
			return
		case data := <-m.incoming:
			m.muPending.Lock()
			if pdebug.Enabled {
				pdebug.Printf("background reader: received %d more bytes, appending", len(data))
			}
			m.pending = append(m.pending, data...)
			m.updateBufsize(len(m.pending))
			m.muPending.Unlock()

			// Wake up the writer goroutine
			m.cond.Broadcast()
		}
	}
}

func (m *minion) runWriter(ctx context.Context) {
	if pdebug.Enabled {
		defer pdebug.Printf("background writer: exiting")
	}
	defer close(m.done)

	// This goroutine waits for the receiver goroutine to wake
	// it up. When it's awake, we know that there's at least one
	// piece of data to send to the fluentd server.
	var conn net.Conn
	defer func() {
		if conn != nil {
			if pdebug.Enabled {
				pdebug.Printf("background writer: closing connection (in cleanup)")
			}
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

		m.cond.L.Lock()
		for {
			m.muPending.RLock()
			pendingLen := len(m.pending)
			m.muPending.RUnlock()
			if pendingLen > 0 {
				if pdebug.Enabled {
					pdebug.Printf("background writer: %d bytes to write", pendingLen)
				}
				break
			}

			select {
			case <-ctx.Done():
				return
			default:
			}

			m.cond.Wait()
		}
		m.cond.L.Unlock()

		// if we're not connected, we should do that now.
		// there are two cases where we can get to this point.
		// 1. reader got something, want us to write
		// 2. reader got notified of cancel, want us to exit
		// case 1 is simple. in case 2, we need to at least attempt to
		// flush the remaining buffer, without checking the context cancelation
		// status, otherwise we exit immediately

		m.muFlush.RLock()
		flush := m.flush
		m.muFlush.RUnlock()

		if pdebug.Enabled {
			if flush {
				pdebug.Printf("background writer: in flush mode")
			}
		}

		if conn == nil {
			var dialer net.Dialer
			if flush {
				for conn == nil {
					conn, _ = dialer.Dial(m.network, m.address)
				}
			} else {
				connCtx, cancel := context.WithTimeout(ctx, m.dialTimeout)
				for conn == nil {
					select {
					case <-connCtx.Done():
						cancel()
						return
					default:
						conn, _ = dialer.DialContext(connCtx, m.network, m.address)
					}
				}
				cancel()
			}

			if pdebug.Enabled {
				pdebug.Printf("background writer: connected to %s:%s", m.network, m.address)
			}
		}

		if flush {
			conn.SetWriteDeadline(time.Time{})
		} else {
			conn.SetWriteDeadline(time.Now().Add(m.writeTimeout))
		}

		m.muPending.Lock()

		if pdebug.Enabled {
			pdebug.Printf("background writer: attempting to write %d bytes", len(m.pending))
		}

		n, err := conn.Write(m.pending)
		if pdebug.Enabled {
			pdebug.Printf("background writer: wrote %d bytes", n)
		}

		if err != nil {
			if pdebug.Enabled {
				pdebug.Printf("background writer: error while writing: %s", err)
			}
			conn.Close()
			conn = nil
		} else {
			m.updateBufsize(len(m.pending) - n)
			m.pending = m.pending[n:]
			if pdebug.Enabled {
				pdebug.Printf("%d more bytes to write", len(m.pending))
			}
		}
		m.muPending.Unlock()
	}
}
