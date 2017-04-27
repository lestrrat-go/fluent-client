package fluent_test

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	fluent "github.com/lestrrat/go-fluent-client"
	"github.com/stretchr/testify/assert"
)

func TestPost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dir, err := ioutil.TempDir("", "sock-")
	if !assert.NoError(t, err, "created temporary dir") {
		return
	}
	defer os.RemoveAll(dir)

	file := filepath.Join(dir, "TestPost.sock")

	l, err := net.Listen("unix", file)
	if !assert.NoError(t, err, "listen to unix socket") {
		return
	}

	serverDone := make(chan struct{})
	serverReady := make(chan struct{})
	r := make(chan []byte)
	go func() {
		defer close(r)
		defer close(serverDone)
		var once sync.Once
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			once.Do(func() { close(serverReady) })
			conn, err := l.Accept()
			if err != nil {
				t.Logf("Failed to accept: %s", err)
				return
			}

			for {
				conn.SetReadDeadline(time.Now().Add(time.Second))
				var v []interface{}
				if err := json.NewDecoder(conn).Decode(&v); err != nil {
					return
				}
				buf, _ := json.Marshal(v)
				r <- buf
			}
		}
	}()

	<-serverReady
	client, err := fluent.New(
		fluent.WithNetwork("unix"),
		fluent.WithAddress(file),
		fluent.WithJSONMarshaler(),
	)
	if !assert.NoError(t, err, "failed to create fluent client") {
		return
	}
	defer client.Shutdown(nil)

	var testcases = []struct {
		in  map[string]string
		out string
	}{
		{
			map[string]string{"foo": "bar"},
			`["tag_name",1482493046,{"foo":"bar"},null]`,
		},
		{
			map[string]string{"fuga": "bar", "hoge": "fuga"},
			`["tag_name",1482493046,{"fuga":"bar","hoge":"fuga"},null]`,
		},
	}

	for _, data := range testcases {
		err := client.Post("tag_name", data.in, fluent.WithTimestamp(time.Unix(1482493046, 0)))
		if !assert.NoError(t, err, "client.Post should succeed") {
			return
		}

		if !assert.Equal(t, data.out, string(<-r)) {
			return
		}
	}
	client.Shutdown(nil)
	l.Close()

	select {
	case <-ctx.Done():
		t.Logf("context canceled: %s", ctx.Err())
	case <-serverDone:
		t.Logf("server exited")
	}
}
