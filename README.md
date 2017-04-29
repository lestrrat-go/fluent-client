# go-fluent-client

A fluentd client

[![Build Status](https://travis-ci.org/lestrrat/go-fluent-client.png?branch=master)](https://travis-ci.org/lestrrat/go-fluent-client)

[![GoDoc](https://godoc.org/github.com/lestrrat/go-fluent-client?status.svg)](https://godoc.org/github.com/lestrrat/go-fluent-client)

# DESCRIPTION

```go
package fluent_test

import (
  "context"
  "log"
  "time"

  fluent "github.com/lestrrat/go-fluent-client"
)

func Example() {
  // Connects to fluentd at 127.0.0.1:24224. If you want to connect to
  // a different host, use the following:
  //
  //   client, err := fluent.New(fluent.WithAddress("fluent.example.com"))
  //
  client, err := fluent.New()
  if err != nil {
    // fluent.New may return an error if invalid values were
    // passed to the constructor
    log.Printf("failed to create client: %s", err)
    return
  }

  // do not forget to shutdown this client at the end. otherwise
  // we would not know if we were able to flush the pending
  // buffer or not.
  defer func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := client.Shutdown(ctx); err != nil {
      // Failed to shutdown properly. force-close it
      client.Close()
    }
  }()

  var payload = map[string]string{
    "foo": "bar",
  }
  if err := client.Post("tag", payload); err != nil {
    log.Printf("failed to post: %s", err)
    return
  }
}
```

# BENCHMARKS

instructions: make sure you have the required fluentd clients, start fluentd at 127.0.0.1:24224, and run

```
go test -run=none -bench=. -benchmem -tags bench
```

```
BenchmarkK0kubun-4    	 1000000	      2576 ns/op	     976 B/op	      13 allocs/op
BenchmarkLestrrat-4   	  500000	      2651 ns/op	     574 B/op	       6 allocs/op
BenchmarkFluent-4     	  200000	      9163 ns/op	     904 B/op	      10 allocs/op
PASS
ok  	github.com/lestrrat/go-fluent-client	5.937s
```

## Versions

| Library | Version |
|---------|---------|
| github.com/lestrrat/go-fluent-client | d39385acb076df42a37322fb911fbcbfa9ceaf8b |
| github.com/k0kubun/fluent-logger-go | e1cfc57bb12c99d7207d43b942527c9450d14382 |
| github.com/fluent/fluent-logger-golang | b8d749d6b17d9373c54c9f66b1f1c075a83bbfed |

## Analysis 

### github.com/lestrrat/go-fluent-client

#### Pros

I'm biased (duh): 

1. "proper" `Shutdown` method to flush buffers at the end
2. Tried very hard to avoid any race conditions

#### Cons

Very, very new and untested on the field.

With all the trickery, still can't beat `github.com/k0kubun/fluent-logger-go` in benchmarks.

### github.com/k0kubun/fluent-logger-go

#### Pros

This library consistently records the shorted wallclock time per iteration. I believe this is due to the
fact that it does very little error handling and synchronization. If you use the msgpack serialization
format and that's it, you probably will be fine using this library.

#### Cons

Do note that as of the version I tested above, the `Logger.Log` method has a glaring race condition
that will probably corrupt your messages sooner than you can blink: DO NOT USE THE `Logger.Log` method.

Also, there is no way for the caller to check for serialization errors when using `Logger.Post`. You can get the
error status using `Logger.Log`, but as previously stated, you do not want to use it. 

Finally, there is no way to flush pending buffers: If you append a lot of buffers in succession, and
abruptly quit your program, you're done for. You lose all your data.

Oh, and it supports JSON only, but this is a very minor issue: a casual user really shouldn't have to care which
serialization format you're sending your format with.

### github.com/fluent/fluent-logger-golang

#### Pros

This official binding from the maitainers of fluentd is by far the most battle-tested library. It may be
a bit slow, but it's sturdy, period.

#### Cons

The benchmark scores are pretty low. This could just be my benchmark, so please take with a grain of salt.

Looking at the code, it looks non-gopher-ish. Use of `panic` is one such item. In Go you should avoid
casual panics, which causes long-running daemons to write code like this https://github.com/moby/moby/blob/1325f667eeb42b717c2f9d369f2ee6d701a280e3/daemon/logger/fluentd/fluentd.go#L46-L49



