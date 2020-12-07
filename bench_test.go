package sse

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
)

type mockResponseWriter struct {
	c chan bool
}

func (m *mockResponseWriter) Header() (h http.Header) {
	return http.Header{}
}

func (m *mockResponseWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (m *mockResponseWriter) WriteString(s string) (n int, err error) {
	return len(s), nil
}

func (m *mockResponseWriter) WriteHeader(int) {}

func (m *mockResponseWriter) Flush() {}

func BenchmarkServeHTTP(b *testing.B) {
	srv := NewServer(&Options{
		Logger: nil,
	})

	defer srv.Shutdown(context.TODO())

	ctx, done := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", "/channel-name", nil)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		res := &mockResponseWriter{
			make(chan bool),
		}

		go func() {
			done()
		}()

		srv.ServeHTTP(res, req)
	}
}

func BenchmarkSendMessage(b *testing.B) {
	srv := NewServer(&Options{
		Logger: nil,
	})

	defer srv.Shutdown(context.TODO())

	for n := 0; n < 10; n++ {
		srv.addChannel(fmt.Sprintf("CH-%d", n+1))
	}

	wgReg := sync.WaitGroup{}
	wgCh := sync.WaitGroup{}

	for n := 0; n < 10; n++ {
		for name, ch := range srv.channels {
			wgReg.Add(1)
			wgCh.Add(1)

			name := name
			ch := ch
			go func() {
				c := newClient(name)
				ch.addClient(c)

				wgReg.Done()

				for range c.send {
				}

				wgCh.Done()
			}()
		}
	}

	wgReg.Wait()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		srv.SendMessage("", SimpleMessage("hello"))
	}

	srv.close()

	wgCh.Wait()
}
