// +build bench

package fluent_test

import (
	"testing"

	official "github.com/fluent/fluent-logger-golang/fluent"
	k0kubun "github.com/k0kubun/fluent-logger-go"
	lestrrat "github.com/lestrrat/go-fluent-client"
)

const tag = "debug.test"
const postsPerIter = 1

func BenchmarkK0kubun(b *testing.B) {
	c := k0kubun.NewLogger(k0kubun.Config{})
	for i := 0; i < b.N; i++ {
		for j := 0; j < postsPerIter; j++ {
			c.Post(tag, map[string]interface{}{"count": j})
		}
	}
}

func BenchmarkLestrrat(b *testing.B) {
	c, _ := lestrrat.New()
	for i := 0; i < b.N; i++ {
		for j := 0; j < postsPerIter; j++ {
			if c.Post(tag, map[string]interface{}{"count": j}) != nil {
				b.Logf("whoa Post failed")
			}
		}
	}
	c.Shutdown(nil)
}

func BenchmarkLestrratJSON(b *testing.B) {
	c, _ := lestrrat.New(lestrrat.WithJSONMarshaler())
	for i := 0; i < b.N; i++ {
		for j := 0; j < postsPerIter; j++ {
			if c.Post(tag, map[string]interface{}{"count": j}) != nil {
				b.Logf("whoa Post failed")
			}
		}
	}
	c.Shutdown(nil)
}

func BenchmarkLestrratUnbuffered(b *testing.B) {
	c, _ := lestrrat.New(lestrrat.WithBuffered(false))
	for i := 0; i < b.N; i++ {
		for j := 0; j < postsPerIter; j++ {
			if c.Post(tag, map[string]interface{}{"count": j}) != nil {
				b.Logf("whoa Post failed")
			}
		}
	}
	c.Shutdown(nil)
}

func BenchmarkOfficial(b *testing.B) {
	c, _ := official.New(official.Config{})
	for i := 0; i < b.N; i++ {
		for j := 0; j < postsPerIter; j++ {
			if c.Post(tag, map[string]interface{}{"count": j}) != nil {
				b.Logf("whoa Post failed")
			}
		}
	}
	c.Close()
}

func BenchmarkOfficialJSON(b *testing.B) {
	c, _ := official.New(official.Config{MarshalAsJSON: true})
	for i := 0; i < b.N; i++ {
		for j := 0; j < postsPerIter; j++ {
			if c.Post(tag, map[string]interface{}{"count": j}) != nil {
				b.Logf("whoa Post failed")
			}
		}
	}
	c.Close()
}

