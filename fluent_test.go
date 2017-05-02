package fluent_test

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	fluent "github.com/lestrrat/go-fluent-client"
	pdebug "github.com/lestrrat/go-pdebug"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	msgpack "gopkg.in/vmihailenco/msgpack.v2"
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
	defer pdebug.Printf("bail out of server.Run")
	defer close(s.done)

	go func() {
		select {
		case <-ctx.Done():
			s.listener.Close()
		}
	}()

	pdebug.Printf("server started")
	var once sync.Once
	for {
		pdebug.Printf("server loop")
		select {
		case <-ctx.Done():
			if pdebug.Enabled {
				pdebug.Printf("cancel detected in server.Run")
			}
			return
		default:
		}

		once.Do(func() { close(s.ready) })
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		var dec func(interface{}) error
		if s.useJSON {
			dec = json.NewDecoder(conn).Decode
		} else {
			msgpdec := msgpack.NewDecoder(conn)
			dec = func(v interface{}) error {
				return msgpdec.Decode(v)
			}
		}

		readerCh := make(chan *fluent.Message)
		go func(ch chan *fluent.Message) {
			for {
				conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				var v fluent.Message
				if err := dec(&v); err != nil {
					var decName string
					if s.useJSON {
						decName = "json"
					} else {
						decName = "msgpack"
					}
					log.Printf("failed to decode %s: %s", decName, err)
					if errors.Cause(err) == io.EOF {
						return
					}
					continue
				}
				select {
				case <-ctx.Done():
					return
				case ch <- &v:
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
			client.Shutdown(nil)
			scancel()
		})
	}
}

func TestPostRoundtrip(t *testing.T) {
	var testcases = []interface{}{
		map[string]interface{}{"foo": "bar"},
		map[string]interface{}{"fuga": "bar", "hoge": "fuga"},
	}

	for _, marshalerName := range []string{"json", "msgpack", "msgpack-subsecond"} {
		var useJSON bool
		var options []fluent.Option
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
				}, options...)...
			)
			if !assert.NoError(t, err, "failed to create fluent client") {
				return
			}
			defer client.Shutdown(nil)

			for _, data := range testcases {
				err := client.Post("tag_name", data, fluent.WithTimestamp(time.Unix(1482493046, 0)))
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
				if !assert.Equal(t, &fluent.Message{Tag: "tag_name", Time: fluent.EventTime{ Time: time.Unix(1482493046, 0) }, Record: data}, s.Payload[i]) {
					return
				}
			}

		})
	}
}
