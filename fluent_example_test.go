package fluent_test

import (
	"context"
	"log"
	"time"

	fluent "github.com/lestrrat/go-fluent-client"
)

func Example() {
  // Connects to fluentd at fluent.example.com:24224. If you are
  // connecting to 127.0.0.1:24224, you can call `New()` without
  // any arguments
	client, err := fluent.New(fluent.WithAddress("fluent.example.com"))
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

	var payload interface{}
	if err := client.Post("tag", payload); err != nil {
		log.Printf("failed to post: %s", err)
		return
	}
}
