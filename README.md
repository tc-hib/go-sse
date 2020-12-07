# go-sse

Server-Sent Events for Go

This is a fork of [alexandrevicenzi/go-sse](https://github.com/alexandrevicenzi/go-sse) made for my own requirements.

You probably want to use the original.

*WARNING* This fork still has to be tested!

## Installation

`go get github.com/tc-hib/go-sse`

## Example

Server side:

```go
package main

import (
    "log"
    "net/http"
    "time"

    "github.com/tc-hib/go-sse"
)

func main() {
    // Create SSE server
    s := sse.NewServer(nil)

    // Configure the route
    http.Handle("/events/", s)

    // Send messages every 5 seconds
    go func() {
        for {
            s.SendMessage("/events/my-channel", sse.SimpleMessage(time.Now().Format("2006/02/01/ 15:04:05")))
            time.Sleep(5 * time.Second)
        }
    }()

    log.Println("Listening at :3000")
    http.ListenAndServe(":3000", nil)
}
```

Client side (JavaScript):

```js
e = new EventSource('/events/my-channel');
e.onmessage = function(event) {
    console.log(event.data);
};
```

## License

MIT
