package sse

import (
	"sync"
)

// Channel represents a server sent events channel.
type Channel struct {
	mu      sync.RWMutex
	name    string
	clients map[*client]struct{}
}

func newChannel(name string) *Channel {
	return &Channel{
		mu:      sync.RWMutex{},
		name:    name,
		clients: make(map[*client]struct{}),
	}
}

// SendMessage broadcasts a message to all clients in a channel.
// Safe to use from any goroutine.
func (c *Channel) SendMessage(message *Message) {
	c.mu.RLock()

	for client := range c.clients {
		client.sendMessage(message)
	}

	c.mu.RUnlock()
}

// close closes the channel and disconnects all clients.
func (c *Channel) close() {
	var clients map[*client]struct{}

	c.mu.Lock()

	clients = c.clients
	c.clients = make(map[*client]struct{})

	c.mu.Unlock()

	for client := range clients {
		client.close()
	}
}

// ClientCount returns the number of clients connected to this channel.
func (c *Channel) ClientCount() int {
	c.mu.RLock()
	count := len(c.clients)
	c.mu.RUnlock()

	return count
}

// Name returns the channel's name.
func (c *Channel) Name() string {
	return c.name
}

func (c *Channel) addClient(client *client) {
	c.mu.Lock()
	c.clients[client] = struct{}{}
	c.mu.Unlock()
}

func (c *Channel) removeClient(client *client) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	client.close()
	delete(c.clients, client)

	return len(c.clients)
}
