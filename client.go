package sse

import "sync"

// client represents a web browser connection.
type client struct {
	mu      sync.Mutex
	channel string
	send    chan *Message
	removed chan struct{}
	closed  bool
}

func newClient(channel string) *client {
	return &client{
		channel: channel,
		send:    make(chan *Message, 1),
		removed: make(chan struct{}, 1),
	}
}

// sendMessage sends a message to client.
func (c *client) sendMessage(message *Message) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	c.send <- message
}

// close closes a client.
func (c *client) close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	c.closed = true
	close(c.send)

	// If the buffer is full, we don't care because it is made to be read only once
	// Must not happen anyway
	select {
	case c.removed <- struct{}{}:
	default:
	}
}
