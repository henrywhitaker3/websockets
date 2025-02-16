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

## Message Replies

You can pass the `websockets.WithReply(&out)` when you expect a reply back for
the message:

```go
server := websockets.NewServer(websockets.ServerOpts{})
...

type replyHandler struct{
    with string
}

func (r replyHandler) Empty() any {
    return ""
}

func (r replyHandler) Handle(conn websockets.Connection, _ any) error {
    return conn.Send(conn.Context(), r.with)
}

server.Register(websockets.Topic("bongo"), replyHandler{"pear"})

var out string
client.Send(ctx, websockets.Topic("bongo"), nil, websockets.WithReply(&out))
fmt.Println(out) // Prints: pear
```

## Acknowledgements

You can wait for the server to acknowledge the message by passing the
`websockets.WithAck()` flag:

```go
client.Send(ctx, websockets.Topic("bongo"), nil, websockets.WithAck())
```
