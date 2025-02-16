package websockets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn *websocket.Conn

	handlers   map[Topic]Handler
	handlersMu *sync.RWMutex

	tx chan *message

	handling *sync.WaitGroup
	pipes    map[string]chan *message

	fin      chan struct{}
	closer   *sync.Once
	isClosed bool

	logger Logger
	opts   ClientOpts
}

type ClientOpts struct {
	Addr string

	// Headers to use when connecting to the server
	Headers http.Header

	// The length of time to wait for a reply from the server (optional)
	// defaults to 1s
	ReplyTimeout time.Duration

	Logger Logger
}

func NewClient(opts ClientOpts) (*Client, error) {
	conn, resp, err := websocket.DefaultDialer.Dial(opts.Addr, opts.Headers)
	if err != nil {
		if resp == nil {
			return nil, fmt.Errorf("connect error: %w", err)
		}
		return nil, fmt.Errorf("connect error: %w: %d", err, resp.StatusCode)
	}
	if opts.Logger == nil {
		opts.Logger = nilLogger{}
	}
	if opts.ReplyTimeout == 0 {
		opts.ReplyTimeout = time.Second
	}
	opts.Addr = strings.ReplaceAll(opts.Addr, "http://", "ws://")
	opts.Addr = strings.ReplaceAll(opts.Addr, "https://", "wss://")
	c := &Client{
		conn:       conn,
		handlers:   map[Topic]Handler{},
		handlersMu: &sync.RWMutex{},
		tx:         make(chan *message, 250),
		handling:   &sync.WaitGroup{},
		pipes:      map[string]chan *message{},
		fin:        make(chan struct{}),
		closer:     &sync.Once{},
		logger:     opts.Logger,
		opts:       opts,
	}
	go c.readPump()
	go c.writePump()
	return c, nil
}

// Register handlers for topics originating from the server
func (c *Client) Register(topic Topic, h Handler) error {
	if slices.Contains(protectedTopics, topic) {
		return errors.New("cannot register protected topic name")
	}
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	c.handlers[topic] = h
	return nil
}

// Sends a message to the server and unmarshalls it into output
func (c *Client) Send(ctx context.Context, topic Topic, content any, flags ...Flag) error {
	if c.isClosed {
		return errors.New("send on closed connection")
	}
	msg, err := newMessage(topic, content)
	if err != nil {
		return err
	}
	for _, flag := range flags {
		if err := flag.ModifyMessage(msg); err != nil {
			return fmt.Errorf("apply flag to message: %w", err)
		}
	}

	if !msg.ShouldAck && !msg.ShouldReply && !msg.ShouldSucceed {
		return c.sendForget(ctx, msg)
	}

	count := 0
	if msg.ShouldAck {
		count++
	}
	if msg.ShouldReply {
		count++
	}
	if msg.ShouldSucceed {
		count++
	}
	replies, err := c.sendReply(ctx, msg, count)
	if err != nil {
		return err
	}
	if len(replies) != count && replies[len(replies)-1].Topic != errorT {
		return fmt.Errorf("expected %d replies, got %d", count, len(replies))
	}

	for i, reply := range replies {
		if i == 0 && msg.ShouldAck {
			if reply.Topic != ack {
				return errors.New("did not get an ack")
			}
			continue
		}
		if i == len(replies)-1 && msg.ShouldSucceed {
			fmt.Printf("should succed got %s\n", reply.Topic)
			if !slices.Contains([]Topic{success, errorT}, reply.Topic) {
				return errors.New("did not got success or error reply")
			}
			if reply.Topic == success {
				return nil
			}
			errMsg := ""
			if err := json.Unmarshal(reply.Content, &errMsg); err != nil {
				return fmt.Errorf("unmarshal error message: %w", err)
			}
			return errors.New(errMsg)
		}
		if err := json.Unmarshal(reply.Content, msg.replyTarget); err != nil {
			return fmt.Errorf("unmarshal reply content: %w", err)
		}
	}
	return nil
}

func (c *Client) sendReply(ctx context.Context, msg *message, count int) ([]*message, error) {
	if len(msg.Id) == 0 {
		return nil, errors.New("reply message must contain id")
	}
	pipe := make(chan *message, count)
	c.pipes[string(msg.Id)] = pipe
	defer delete(c.pipes, string(msg.Id))
	c.tx <- msg

	messages := []*message{}

	for range count {
		ctx, cancel := context.WithTimeout(ctx, c.opts.ReplyTimeout)
		defer cancel()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case msg := <-pipe:
			messages = append(messages, msg)
		}
	}
	return messages, nil
}

func (c *Client) sendForget(ctx context.Context, msg *message) error {
	c.tx <- msg
	return nil
}

func (c *Client) readPump() {
	for {
		select {
		case <-c.Closed():
			return
		default:
			var msg message
			if err := c.conn.ReadJSON(&msg); err != nil {
				if websocket.IsUnexpectedCloseError(err) {
					c.close()
					c.logger.Errorf("read on unexpected closed connection", "error", err)
					return
				}
				c.logger.Errorf("reading incoming json: %v", err)
				continue
			}

			if slices.Contains([]Topic{ack, reply, success, errorT}, msg.Topic) {
				pipe, ok := c.pipes[string(msg.Id)]
				if !ok {
					c.logger.Errorf("ack or reply has no recevier")
					continue
				}
				pipe <- &msg
				continue
			}

			if msg.ShouldAck {
				ctx, cancel := context.WithTimeout(context.Background(), c.opts.ReplyTimeout)
				if err := c.sendForget(ctx, &message{Id: msg.Id, Topic: ack}); err != nil {
					cancel()
					c.logger.Errorf("sending ack reply: %w", err)
					continue
				}
				cancel()
			}

			c.handlersMu.RLock()
			handler, ok := c.handlers[msg.Topic]
			c.handlersMu.RUnlock()
			if ok {
				c.handling.Add(1)
				go func() {
					defer c.handling.Done()
					body := handler.Empty()
					if err := json.Unmarshal(msg.Content, &body); err != nil {
						c.logger.Errorf("unmarshal message content: %v", err)
						return
					}
					conn, err := newClientConnection(c, &msg)
					if err != nil {
						c.logger.Errorf("could not create client connection: %w", "error", err)
						return
					}
					if err := handler.Handle(conn, body); err != nil {
						c.logger.Errorf("handler returned error: %v", err)
					}
				}()
				continue
			}

			c.logger.Errorf("unhandled message for %s", msg.Topic)
		}
	}
}

// Returns a new context for use in handlers
func (c *Client) Context() context.Context {
	return context.Background()
}

func (c *Client) writePump() {
	for {
		select {
		case <-c.Closed():
			return
		case msg := <-c.tx:
			if err := c.conn.WriteJSON(msg); err != nil {
				// Maybe log the error?
				c.close()
				return
			}
		}
	}
}

func (c *Client) Close() error {
	c.handling.Wait()
	c.close()
	return c.conn.Close()
}

func (c *Client) Closed() <-chan struct{} {
	return c.fin
}

func (c *Client) close() {
	c.closer.Do(func() {
		close(c.fin)
		c.isClosed = true
	})
}

// A blocking method that waits for the connection to close, or
// for the provided context to be cancelled
func (c *Client) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.Closed():
		return nil
	}
}
