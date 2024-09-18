package fluent_test

import (
	"context"
	"log"
	"time"

	fluent "github.com/lestrrat-go/fluent-client"
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
			log.Printf("Failed to shutdown properly. force-close it")
			client.Close()
		}
		log.Printf("shutdown complete")
	}()

	var payload = map[string]string{
		"foo": "bar",
	}
	log.Printf("Posting message")
	if err := client.Post("debug.test", payload); err != nil {
		log.Printf("failed to post: %s", err)
		return
	}

	// OUTPUT:
}

func ExamplePing() {
	client, err := fluent.New()
	if err != nil {
		log.Printf("failed to create client: %s", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Goroutine to wait for errors
	errorCh := make(chan error, 1)
	go func() {
		// This is just an example to stop pinging on errors
		for {
			select {
			case <-ctx.Done():
				return
			case e := <-errorCh:
				log.Printf("got an error during ping: %s", e.Error())
				cancel()
				return
			}
		}
	}()

	go fluent.Ping(ctx, client, "ping", "hostname", fluent.WithPingResultChan(errorCh))
	// Do what you need with your main program...

	// OUTPUT:
}
