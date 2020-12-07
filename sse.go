package sse

import (
	"context"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
)

// Server represents a server sent events server.
type Server struct {
	mu               sync.RWMutex
	options          *Options
	channels         map[string]*Channel
	addClient        chan *client
	removeClient     chan *client
	closeChannel     chan string
	shutdown         chan struct{}
	shutdownComplete chan struct{}
	closed           bool
}

// WelcomeMaker is a callback function that may provide messages on client (re)connection.
// It must be goroutine-safe.
type WelcomeMaker func(channel string, lastEventID string) []Message

// NewServer creates a new SSE server.
func NewServer(options *Options) *Server {
	if options == nil {
		options = &Options{
			Logger: log.New(os.Stdout, "go-sse: ", log.LstdFlags),
		}
	}

	if options.Logger == nil {
		options.Logger = log.New(ioutil.Discard, "", log.LstdFlags)
	}

	s := &Server{
		mu:               sync.RWMutex{},
		options:          options,
		channels:         make(map[string]*Channel),
		addClient:        make(chan *client),
		removeClient:     make(chan *client),
		shutdown:         make(chan struct{}),
		shutdownComplete: make(chan struct{}),
		closeChannel:     make(chan string),
	}

	go s.dispatch()

	return s
}

func (s *Server) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if s.isClosed() {
		http.Error(response, "SSE server has been shut down", http.StatusInternalServerError)
		return
	}

	flusher, ok := response.(http.Flusher)

	if !ok {
		http.Error(response, "Streaming unsupported.", http.StatusInternalServerError)
		return
	}

	h := response.Header()

	if s.options.hasHeaders() {
		for k, v := range s.options.Headers {
			h.Set(k, v)
		}
	}

	if request.Method == http.MethodOptions {
		response.WriteHeader(http.StatusAccepted)
		return
	}
	if request.Method != http.MethodGet {
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")

	var channelName string

	if s.options.ChannelNameFunc == nil {
		channelName = request.URL.Path
	} else {
		channelName = s.options.ChannelNameFunc(request)
	}

	lastEventID := request.Header.Get("Last-Event-ID")
	c := newClient(channelName)
	s.addClient <- c
	closeNotify := request.Context().Done()

	go func() {
		<-closeNotify
		// We can be there for 2 reasons:
		// - The connection was terminated from the "outside"
		// - The client was removed from the inside (and its stream got closed)
		//
		// In the first case, we've got to notify the SSE server this client shouldn't be listed anymore.
		// This is s.removeClient <- c
		//
		// In the second case we should not wait on s.removeClient.
		// To know we're in the second case, a 1 entry buffered channel is filled when the client is removed.
		// This is <-c.removed
		select {
		case s.removeClient <- c:
		case <-c.removed:
		}
	}()

	response.WriteHeader(http.StatusOK)
	flusher.Flush()

	err := writeMessages(response, s.welcomeMessages(channelName, lastEventID))
	if err != nil {
		s.options.Logger.Println(err)
		response.WriteHeader(http.StatusInternalServerError)
		return
	}
	flusher.Flush()

	for msg := range c.send {
		_, err := response.Write(msg.Bytes())
		if err != nil {
			s.options.Logger.Println(err)
			response.WriteHeader(http.StatusInternalServerError)
			return
		}
		flusher.Flush()
	}
}

// SendMessage broadcasts a message to all clients in a channel.
// If channelName is an empty string, it will broadcast the message to all channels.
func (s *Server) SendMessage(channelName string, message *Message) {
	if s.isClosed() {
		return
	}

	if channelName == "" {
		s.options.Logger.Print("broadcasting message to all channels.")
		for _, ch := range s.Channels() {
			ch.SendMessage(message)
		}
	} else if ch, ok := s.getChannel(channelName); ok {
		ch.SendMessage(message)
		s.options.Logger.Printf("message sent to channel '%s'.", channelName)
	} else {
		s.options.Logger.Printf("message not sent because channel '%s' has no clients.", channelName)
	}
}

// Restart closes all channels and clients and allows new connections.
func (s *Server) Restart() {
	if s.isClosed() {
		return
	}

	s.options.Logger.Print("restarting server.")
	for _, c := range s.ChannelNames() {
		s.CloseChannel(c)
	}
}

// Shutdown performs a graceful server shutdown.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.isClosed() {
		return errors.New("shutdown called twice")
	}

	s.shutdown <- struct{}{}
	select {
	case <-s.shutdownComplete:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// ClientCount returns the number of clients connected to this server.
func (s *Server) ClientCount() int {
	i := 0

	s.mu.RLock()

	for _, channel := range s.channels {
		i += channel.ClientCount()
	}

	s.mu.RUnlock()

	return i
}

// HasChannel returns true if the channel associated with name exists.
func (s *Server) HasChannel(name string) bool {
	_, ok := s.getChannel(name)
	return ok
}

// GetChannel returns the channel associated with name or nil if not found.
func (s *Server) GetChannel(name string) (*Channel, bool) {
	return s.getChannel(name)
}

// Channels returns a list of all channels.
func (s *Server) Channels() []*Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	channels := make([]*Channel, 0, len(s.channels))
	for _, ch := range s.channels {
		channels = append(channels, ch)
	}

	return channels
}

// ChannelNames returns a list of all channel names.
func (s *Server) ChannelNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.channels))
	for name := range s.channels {
		names = append(names, name)
	}

	return names
}

// CloseChannel closes a channel.
func (s *Server) CloseChannel(name string) {
	if s.isClosed() {
		return
	}

	s.closeChannel <- name
}

func (s *Server) isClosed() bool {
	s.mu.RLock()
	closed := s.closed
	s.mu.RUnlock()

	if closed {
		s.options.Logger.Println("server is shut down.")
	}
	return closed
}

func (s *Server) addChannel(name string) *Channel {
	ch := newChannel(name)

	s.mu.Lock()
	s.channels[ch.name] = ch
	s.mu.Unlock()

	s.options.Logger.Printf("channel '%s' created.", ch.name)

	return ch
}

func (s *Server) removeChannel(ch *Channel) {
	s.mu.Lock()
	delete(s.channels, ch.name)
	s.mu.Unlock()

	ch.close()

	s.options.Logger.Printf("channel '%s' closed.", ch.name)
}

func (s *Server) getChannel(name string) (*Channel, bool) {
	s.mu.RLock()
	ch, ok := s.channels[name]
	s.mu.RUnlock()
	return ch, ok
}

// close closes all the channels and removes them.
func (s *Server) close() {
	var channels map[string]*Channel

	s.mu.Lock()

	channels = s.channels
	s.channels = make(map[string]*Channel)

	s.mu.Unlock()

	for _, ch := range channels {
		ch.close()
	}
}

// dispatch is a dedicated coroutine that treats the following queries:
// - Add/remove client (triggered by the HTTP handler)
// - Everything that relies on adding or removing a client: adding/removing channels, shutting down the server...
//
// This is the only place where adding/removing clients an channels happen.
func (s *Server) dispatch() {
	s.options.Logger.Print("server started.")

	for {
		select {

		// New client connected.
		case c := <-s.addClient:
			ch, exists := s.getChannel(c.channel)

			if !exists {
				ch = s.addChannel(c.channel)
			}

			ch.addClient(c)
			s.options.Logger.Printf("new client connected to channel '%s'.", ch.name)

		// Client disconnected.
		case c := <-s.removeClient:
			if ch, exists := s.getChannel(c.channel); exists {
				count := ch.removeClient(c)
				s.options.Logger.Printf("client disconnected from channel '%s'.", ch.name)

				if count == 0 {
					s.options.Logger.Printf("channel '%s' has no clients.", ch.name)
					s.removeChannel(ch)
				}
			}

		// Close channel and all clients in it.
		case channel := <-s.closeChannel:
			if ch, exists := s.getChannel(channel); exists {
				s.removeChannel(ch)
			} else {
				s.options.Logger.Printf("requested to close nonexistent channel '%s'.", channel)
			}

		// Event Source shutdown.
		case <-s.shutdown:
			s.close()

			s.options.Logger.Print("server stopped.")

			s.shutdownComplete <- struct{}{}
			return
		}
	}
}

// welcomeMessages builds the batch of messages that's sent on client connection.
// This includes the reconnection time (RetryInterval) and/or what's provided by WelcomeFunc.
func (s *Server) welcomeMessages(channel string, lastEventID string) []Message {
	var m []Message
	if s.options.WelcomeFunc != nil {
		m = s.options.WelcomeFunc(channel, lastEventID)
	}
	if s.options.RetryInterval != 0 {
		if len(m) == 0 {
			m = []Message{{retry: s.options.RetryInterval}}
		} else {
			m[0].retry = s.options.RetryInterval
		}
	}
	return m
}
