# Websockets

A simple websockets package providing a wrapper around [melody](https://github.com/olahol/melody)

## Starting a server

```go
server := websockets.NewServer(websockets.ServerOpts{})

http.ListenAnsServe(":17456", server)
```

## Connecting to a server

```go
client, err := websockets.NewClient(websockets.ClientOpts{
    Addr: "ws://localhost:17456"
})
if err != nil {
    panic("failed to connect")
}

client.Send(context.Background(), websockets.Topic("some topic"), "a message")
```

## Message Handlers

All messages sent contain a `Topic`, which is used to route message to a specific
handler. Handlers must satisfy the `Handler` interface:

```go
type Handler interface{
    Empty() any
    Handle(conn Connection, content any)
}
```

The `Empty()` function is used to return an empty type used to unmarshal the
message content into. This unmarshaled value is then passed into `Handle()`.

```go
type Handler struct{}

func (h Handler) Empty() any {
    var val string
    return val
}

func (h Handler) Handle(conn websocket.Connection, content any) error {
    val := content.(string)
    fmt.Println(val)
    return nil
}
```

Handlers can be registered against both the server and the client.

```go
server.Register(websockets.Topic("a topic"), handler)
client.Register(websockets.Topic("another topic"), handler)
```
