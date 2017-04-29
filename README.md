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
BenchmarkK0kubun-4    	   10000	    474927 ns/op	    9765 B/op	     129 allocs/op
BenchmarkLestrrat-4   	   10000	    597923 ns/op	    6040 B/op	      60 allocs/op
BenchmarkFluent-4     	   10000	    683505 ns/op	    9040 B/op	     100 allocs/op
PASS
ok  	github.com/lestrrat/go-fluent-client	17.614s
```

## Versions

| Library | Version |
|---------|---------|
| github.com/lestrrat/go-fluent-client | 187e78dfe13f97194e8c7f1cf922c4b6f89100a5 |
| github.com/k0kubun/fluent-logger-go | e1cfc57bb12c99d7207d43b942527c9450d14382 |
| github.com/fluent/fluent-logger-golang | b8d749d6b17d9373c54c9f66b1f1c075a83bbfed |
