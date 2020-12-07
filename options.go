package sse

import (
	"log"
	"net/http"
)

// Options holds server configurations.
type Options struct {
	// RetryInterval change EventSource default retry interval (milliseconds).
	RetryInterval int

	// Headers allow to set custom headers (useful for CORS support).
	Headers map[string]string

	// ChannelNameFunc allows to create custom channel names.
	// Default channel name is the request path.
	ChannelNameFunc func(*http.Request) string
	// All usage logs end up in Logger

	Logger *log.Logger
	// WelcomeFunc may provide messages on client (re)connection.
	//
	// WARNING: It must be goroutine-safe.

	WelcomeFunc WelcomeMaker
	// ConnectionFunc is a callback. It's called on each connection.
	// Returns false to reject connection.
	//
	// WARNING: It must be goroutine-safe.
	ConnectionFunc ConnectionHandler

	// DisconnectionFunc is a callback. It's called on each disconnection.
	//
	// WARNING: It must be goroutine-safe.
	DisconnectionFunc DisconnectionHandler
}

func (opt *Options) hasHeaders() bool {
	return opt.Headers != nil && len(opt.Headers) > 0
}
