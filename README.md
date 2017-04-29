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
BenchmarkK0kubun-4    	   10000	    457245 ns/op	    9750 B/op	     129 allocs/op
BenchmarkLestrrat-4   	   10000	    825743 ns/op	    6040 B/op	      60 allocs/op
BenchmarkFluent-4     	   10000	    778976 ns/op	    9041 B/op	     100 allocs/op
PASS
ok  	github.com/lestrrat/go-fluent-client	20.673s
```

## Versions

| Library | Version |
|---------|---------|
| github.com/lestrrat/go-fluent-client | b49175826c55bc4a195b5958491e0c57e637ecd7 |
| github.com/k0kubun/fluent-logger-go | e1cfc57bb12c99d7207d43b942527c9450d14382 |
| github.com/fluent/fluent-logger-golang | b8d749d6b17d9373c54c9f66b1f1c075a83bbfed |
