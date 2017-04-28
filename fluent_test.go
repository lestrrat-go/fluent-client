package fluent_test

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	fluent "github.com/lestrrat/go-fluent-client"
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
	defer close(s.done)

	var once sync.Once
	for {
		select {
		case <-ctx.Done():
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

		for {
			conn.SetReadDeadline(time.Now().Add(time.Second))
			var v fluent.Message
			if err := dec(&v); err != nil {
				log.Printf("failed to decode: %s", err)
				return
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
			s.Payload = append(s.Payload, &v)
		}
	}
}

func TestPostRoundtrip(t *testing.T) {
	var testcases = []interface{}{
		map[string]interface{}{"foo": "bar"},
		map[string]interface{}{"fuga": "bar", "hoge": "fuga"},
	}

	for _, marshalerName := range []string{"json", "msgpack"} {
		var marshaler fluent.Option
		var useJSON bool
		switch marshalerName {
		case "json":
			useJSON = true
			marshaler = fluent.WithJSONMarshaler()
		case "msgpack":
			marshaler = fluent.WithMsgpackMarshaler()
		}

		t.Run("marshaler="+marshalerName, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			s, err := newServer(useJSON)
			if !assert.NoError(t, err, "newServer should succeed") {
				return
			}
			defer s.Close()
			go s.Run(ctx)

			<-s.Ready()

			client, err := fluent.New(
				fluent.WithNetwork(s.Network),
				fluent.WithAddress(s.Address),
				marshaler,
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

			time.Sleep(time.Second)
			if !assert.Len(t, s.Payload, len(testcases)) {
				return
			}

			for i, data := range testcases {
				if !assert.Equal(t, &fluent.Message{Tag: "tag_name", Time: 1482493046, Record: data}, s.Payload[i]) {
					return
				}
			}

			select {
			case <-ctx.Done():
				t.Logf("context canceled: %s", ctx.Err())
			case <-s.Done():
				t.Logf("server exited")
			}
		})
	}
}
