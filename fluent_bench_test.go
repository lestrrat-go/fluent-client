// +build bench

package fluent_test

import (
	"testing"

	"github.com/fluent/fluent-logger-golang/fluent"
	lestrrat "github.com/lestrrat/go-fluent-client"
)

const tag = "debug.test"

func BenchmarkLestrrat(b *testing.B) {
	c, _ := lestrrat.New()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 10; j++ {
			if c.Post(tag, map[string]interface{}{"count": j}) != nil {
				b.Logf("whoa Post failed")
			}
		}
	}
	c.Shutdown(nil)
}

func BenchmarkFluent(b *testing.B) {
	c, _ := fluent.New(fluent.Config{})
	for i := 0; i < b.N; i++ {
		for j := 0; j < 10; j++ {
			if c.Post(tag, map[string]interface{}{"count": j}) != nil {
				b.Logf("whoa Post failed")
			}
		}
	}
	c.Close()
}
