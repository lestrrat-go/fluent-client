package fluent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	fluent "github.com/lestrrat/go-fluent-client"
	msgpack "github.com/lestrrat/go-msgpack"
	pdebug "github.com/lestrrat/go-pdebug"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

// to hell with race-conditions. no locking!
type server struct {
	cleanup  func()
	done     chan struct{}
	listener net.Listener
	ready    chan struct{}
	useJSON  bool
	Network  string
	Address  string
	Payload  []*fluent.Message
}

func newServer(useJSON bool) (*server, error) {
	dir, err := ioutil.TempDir("", "sock-")
	if err != nil {
		return nil, errors.Wrap(err, `failed to create temporary directory`)
	}

	file := filepath.Join(dir, "test-server.sock")

	l, err := net.Listen("unix", file)
	if err != nil {
		return nil, errors.Wrap(err, `failed to listen to unix socket`)
	}

	s := &server{
		Network:  "unix",
		Address:  file,
		useJSON:  useJSON,
		done:     make(chan struct{}),
		ready:    make(chan struct{}),
		listener: l,
		cleanup: func() {
			l.Close()
			os.RemoveAll(dir)
		},
	}
	return s, nil
}

func (s *server) Close() error {
	if f := s.cleanup; f != nil {
		f()
	}
	return nil
}

func (s *server) Ready() <-chan struct{} {
	return s.ready
}

func (s *server) Done() <-chan struct{} {
	return s.done
}

func (s *server) Run(ctx context.Context) {
	if pdebug.Enabled {
		defer pdebug.Printf("bail out of server.Run")
	}
	defer close(s.done)

	go func() {
		select {
		case <-ctx.Done():
			if pdebug.Enabled {
				pdebug.Printf("context.Context is done, closing listeners")
			}
			s.listener.Close()
		}
	}()

	if pdebug.Enabled {
		pdebug.Printf("server started")
	}
	var once sync.Once
	for {
		if pdebug.Enabled {
			pdebug.Printf("server loop")
		}
		select {
		case <-ctx.Done():
			if pdebug.Enabled {
				pdebug.Printf("cancel detected in server.Run")
			}
			return
		default:
		}

		once.Do(func() { close(s.ready) })
		readerCh := make(chan *fluent.Message)
		go func(ch chan *fluent.Message) {
			if pdebug.Enabled {
				defer pdebug.Printf("bailing out of server reader")
			}
		ACCEPT:
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				conn, err := s.listener.Accept()
				if err != nil {
					if pdebug.Enabled {
						pdebug.Printf("Failed to accept: %s", err)
					}
					return
				}

				if pdebug.Enabled {
					pdebug.Printf("Accepted new connection")
				}

				var dec func(interface{}) error
				if s.useJSON {
					dec = json.NewDecoder(conn).Decode
				} else {
					dec = msgpack.NewDecoder(conn).Decode
				}

				for {
					if pdebug.Enabled {
						pdebug.Printf("waiting for next message...")
					}
					// conn.SetReadDeadline(time.Now().Add(5 * time.Second))
					var v fluent.Message
					if err := dec(&v); err != nil {
						var decName string
						if s.useJSON {
							decName = "json"
						} else {
							decName = "msgpack"
						}
						if pdebug.Enabled {
							pdebug.Printf("test server: failed to decode %s: %s", decName, err)
						}
						if errors.Cause(err) == io.EOF {
							if pdebug.Enabled {
								pdebug.Printf("test server: EOF detected")
							}
							conn.Close()
							continue ACCEPT
						}
						continue
					}

					if pdebug.Enabled {
						pdebug.Printf("Read new fluet.Message")
					}
					select {
					case <-ctx.Done():
						if pdebug.Enabled {
							pdebug.Printf("bailing out of read loop")
						}
						return
					case ch <- &v:
						if pdebug.Enabled {
							pdebug.Printf("Sent new message to read channel")
						}
					}
				}
			}
		}(readerCh)

		for {
			var v *fluent.Message
			select {
			case <-ctx.Done():
				if pdebug.Enabled {
					pdebug.Printf("bailout")
				}
				return
			case v = <-readerCh:
				if pdebug.Enabled {
					pdebug.Printf("new payload: %#v", v)
				}
			}

			// This is some silly stuff, but msgpack would return
			// us map[interface{}]interface{} instead of map[string]interface{}
			// we force the usage of map[string]interface here, so testing is easier
			switch v.Record.(type) {
			case map[interface{}]interface{}:
				newMap := map[string]interface{}{}
				for key, val := range v.Record.(map[interface{}]interface{}) {
					newMap[key.(string)] = val
				}
				v.Record = newMap
			}
			s.Payload = append(s.Payload, v)
		}
	}
}

func TestConnectOnStart(t *testing.T) {
	for _, buffered := range []bool{true, false} {
		t.Run(fmt.Sprintf("failure case, buffered=%t", buffered), func(t *testing.T) {
			// find a port that is not available (this may be timing dependent)
			var dialer net.Dialer
			var port int = 22412
			for i := 0; i < 1000; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				conn, err := dialer.DialContext(ctx, `net`, fmt.Sprintf(`127.0.0.1:%d`, port))
				cancel()
				if err == nil {
					port++
					conn.Close()
					continue
				}
				break
			}
			client, err := fluent.New(
				fluent.WithAddress(fmt.Sprintf(`127.0.0.1:%d`, port)),
				fluent.WithConnectOnStart(true),
				fluent.WithBuffered(buffered),
			)
			if !assert.Error(t, err, `fluent.New should fail`) {
				client.Close()
				return
			}
		})
	}

	s, err := newServer(false)
	if !assert.NoError(t, err, "newServer should succeed") {
		return
	}
	defer s.Close()

	// This is just to stop the server
	sctx, scancel := context.WithCancel(context.Background())
	defer scancel()

	go s.Run(sctx)

	<-s.Ready()

	for _, buffered := range []bool{true, false} {
		t.Run(fmt.Sprintf("normal case, buffered=%t", buffered), func(t *testing.T) {
			client, err := fluent.New(
				fluent.WithNetwork(s.Network),
				fluent.WithAddress(s.Address),
				fluent.WithConnectOnStart(true),
				fluent.WithBuffered(buffered),
			)
			if !assert.NoError(t, err, "fluent.New should succeed") {
				return
			}
			client.Close()
		})
	}
}

func TestCloseAndPost(t *testing.T) {
	client, err := fluent.New()
	if !assert.NoError(t, err, `fluent.New should succeed`) {
		return
	}

	// immediately close
	client.Close()

	if !assert.Error(t, client.Post("tag_name", nil), `we should error after a call to Close()`) {
		return
	}
}

func TestWithContext(t *testing.T) {
	client, err := fluent.New()
	if !assert.NoError(t, err, `fluent.New should succeed`) {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel early

	// Make sure ctx is closed
	<-ctx.Done()

	if !assert.Error(t, client.Post("tag_name", "Hello, World", fluent.WithContext(ctx)), `we should error after context is canceled`) {
		return
	}
}

func TestTagPrefix(t *testing.T) {
	s, err := newServer(false)
	if !assert.NoError(t, err, "newServer should succeed") {
		return
	}
	defer s.Close()

	// This is just to stop the server
	sctx, scancel := context.WithCancel(context.Background())
	defer scancel()

	go s.Run(sctx)

	<-s.Ready()

	client, err := fluent.New(
		fluent.WithNetwork(s.Network),
		fluent.WithAddress(s.Address),
		fluent.WithTagPrefix("test"),
	)
	if !assert.NoError(t, client.Post("tag_name", map[string]interface{}{"foo": 1}), "Post should succeed") {
		return
	}

	client.Shutdown(nil)

	for _, p := range s.Payload {
		if !assert.Equal(t, "test.tag_name", p.Tag, "tag should have prefix") {
			return
		}
	}
}

func TestBufferFull(t *testing.T) {
	s, err := newServer(false)
	if !assert.NoError(t, err, "newServer should succeed") {
		return
	}
	defer s.Close()

	client, err := fluent.New(
		fluent.WithNetwork(s.Network),
		fluent.WithAddress(s.Address),
		fluent.WithBufferLimit(256),
		fluent.WithWriteThreshold(1),
	)

	count := 1
	// Write until buffer is full.
	if !assert.NoError(t, client.Post("tag_name", map[string]interface{}{"foo": 0}, fluent.WithSyncAppend(true)), "Post should succeed") {
		return
	}
	for i := 1; ; i++ {
		err := client.Post("tag_name", map[string]interface{}{"foo": i}, fluent.WithSyncAppend(true))
		if fluent.IsBufferFull(err) {
			if pdebug.Enabled {
				pdebug.Printf("Detected full buffer. Stopping Post() loop")
			}
			break
		}
		count++
	}

	sctx, scancel := context.WithCancel(context.Background())
	defer scancel()

	// Start the server, which should start the writer process
	go s.Run(sctx)

	<-s.Ready()

	// write one more message
	var wroteOneMore bool
	for i := 1; i < 10; i++ {
		if pdebug.Enabled {
			pdebug.Printf("Writing one more message...")
		}
		err := client.Post("tag_name", map[string]interface{}{"foo": i}, fluent.WithSyncAppend(true))
		if err == nil {
			count++
			wroteOneMore = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !assert.True(t, wroteOneMore, "expected to be able to write one more message") {
		return
	}

	// wait for the server to process all of our messages
	{
		timeout := time.NewTimer(10 * time.Second)
		defer timeout.Stop()
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for loop := true; loop; {
			select {
			case <-timeout.C:
				t.Errorf("timed out while waiting for the server to process requests")
				return // Don't forget to return
			case <-tick.C:
				if len(s.Payload) == count {
					t.Logf("Got expected %d messages in the server", len(s.Payload))
					loop = false
				}
			}
		}
	}

	if pdebug.Enabled {
		pdebug.Printf("Writing one more after draining")
	}
	// See if we can still write after the buffer has been drained
	s.Payload = s.Payload[0:0]
	if !assert.NoError(t, client.Post("tag_name", map[string]interface{}{"foo": 1}, fluent.WithSyncAppend(true)), "writing after the buffer has been drained should succeed") {
		return
	}

	// wait for the server to process all of our messages
	{
		timeout := time.NewTimer(10 * time.Second)
		defer timeout.Stop()
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for loop := true; loop; {
			select {
			case <-timeout.C:
				t.Errorf("timed out while waiting for the server to process requests")
			case <-tick.C:
				if len(s.Payload) == 1 {
					t.Logf("Got expected %d messages in the server", len(s.Payload))
					loop = false
				}
			}
		}
	}

	client.Shutdown(nil)
}

type badmsgpack struct{}

func (msg *badmsgpack) EncodeMsgpack(_ *msgpack.Encoder) error {
	return errors.New(`badmsgpack`)
}

func TestPostSync(t *testing.T) {
	for _, syncAppend := range []bool{true, false} {
		t.Run("sync="+strconv.FormatBool(syncAppend), func(t *testing.T) {
			s, err := newServer(false)
			if !assert.NoError(t, err, "newServer should succeed") {
				return
			}
			defer s.Close()

			// This is just to stop the server
			sctx, scancel := context.WithCancel(context.Background())

			go s.Run(sctx)

			<-s.Ready()

			client, err := fluent.New(
				fluent.WithNetwork(s.Network),
				fluent.WithAddress(s.Address),
				fluent.WithBufferLimit(1),
			)

			if !assert.NoError(t, err, "failed to create fluent client") {
				return
			}

			var options []fluent.Option
			if syncAppend {
				options = append(options, fluent.WithSyncAppend(true))
			}

			err = client.Post("tag_name", map[string]interface{}{"foo": 1}, options...)
			if syncAppend {
				if !assert.Error(t, err, "should receive an error") {
					return
				}
			} else {
				if !assert.NoError(t, err, "should NOT receive an error") {
					return
				}
			}

			err = client.Post("tag_name", &badmsgpack{}, options...)
			if syncAppend {
				if !assert.Error(t, err, "should receive an error") {
					return
				}
			} else {
				if !assert.NoError(t, err, "should NOT receive an error") {
					return
				}
			}
			client.Shutdown(nil)
			scancel()
		})
	}
}

type Payload struct {
	Foo string `msgpack:"foo" json:"foo"`
	Bar string `msgpack:"bar" json:"bar"`
}

func TestPostRoundtrip(t *testing.T) {
	var testcases = []interface{}{
		map[string]interface{}{"foo": "bar"},
		map[string]interface{}{"fuga": "bar", "hoge": "fuga"},
		Payload{Foo: "foo", Bar: "bar"},
		&Payload{Foo: "foo", Bar: "bar"},
		"hogehoge",
	}

	for _, buffered := range []bool{true, false} {
		t.Run(fmt.Sprintf("buffered=%t", buffered), func(t *testing.T) {
			var options []fluent.Option
			if !buffered {
				options = append(options, fluent.WithBuffered(false))
			}
			for _, marshalerName := range []string{"json", "msgpack", "msgpack-subsecond"} {
				var useJSON bool
				switch marshalerName {
				case "json":
					useJSON = true
					options = append(options, fluent.WithJSONMarshaler())
				case "msgpack":
					options = append(options, fluent.WithMsgpackMarshaler())
				case "msgpack-subsecond":
					options = append(options, fluent.WithMsgpackMarshaler())
					options = append(options, fluent.WithSubsecond(true))
				}

				t.Run("marshaler="+marshalerName, func(t *testing.T) {
					// This is used for global cancel/teardown
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()

					s, err := newServer(useJSON)
					if !assert.NoError(t, err, "newServer should succeed") {
						return
					}
					defer s.Close()

					// This is just to stop the server
					sctx, scancel := context.WithCancel(context.Background())
					go s.Run(sctx)

					<-s.Ready()

					client, err := fluent.New(
						append([]fluent.Option{
							fluent.WithNetwork(s.Network),
							fluent.WithAddress(s.Address),
						}, options...)...,
					)
					if !assert.NoError(t, err, "failed to create fluent client") {
						return
					}
					defer client.Shutdown(nil)

					for _, data := range testcases {
						err := client.Post("tag_name", data, fluent.WithTimestamp(time.Unix(1482493046, 0).UTC()))
						if !assert.NoError(t, err, "client.Post should succeed") {
							return
						}
					}
					client.Shutdown(nil)

					time.Sleep(time.Second)
					scancel()
					if pdebug.Enabled {
						pdebug.Printf("canceled server context")
					}

					select {
					case <-ctx.Done():
						t.Errorf("context canceled: %s", ctx.Err())
						return
					case <-s.Done():
					}

					if !assert.Len(t, s.Payload, len(testcases)) {
						return
					}

					for i, data := range testcases {
						// If data is not a map, we would need to convert it
						var payload interface{}
						if _, ok := data.(map[string]interface{}); ok {
							payload = data
						} else {
							var marshaler func(interface{}) ([]byte, error)
							var unmarshaler func([]byte, interface{}) error
							if useJSON {
								marshaler = json.Marshal
								unmarshaler = json.Unmarshal
							} else {
								marshaler = msgpack.Marshal
								unmarshaler = msgpack.Unmarshal
							}
							buf, err := marshaler(data)
							if !assert.NoError(t, err, "Marshal should succeed") {
								return
							}
							if !assert.NoError(t, unmarshaler(buf, &payload), "Unmarshal should succeed") {
								return
							}
						}

						if !assert.Equal(t, &fluent.Message{Tag: "tag_name", Time: fluent.EventTime{Time: time.Unix(1482493046, 0).UTC()}, Record: payload}, s.Payload[i]) {
							return
						}
					}

				})
			}
		})
	}
}
