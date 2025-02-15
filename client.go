package websockets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
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

	// The length of time to wait for an ack from the server (optional)
	AckTimeout time.Duration

	Logger Logger
}

func NewClient(opts ClientOpts) (*Client, error) {
	conn, resp, err := websocket.DefaultDialer.Dial(opts.Addr, opts.Headers)
	if err != nil {
		return nil, fmt.Errorf("%w: %d", err, resp.StatusCode)
	}
	if opts.Logger == nil {
		opts.Logger = nilLogger{}
	}
	c := &Client{
		conn:       conn,
		handlers:   map[Topic]Handler{},
		handlersMu: &sync.RWMutex{},
		tx:         make(chan *message, 250),
		handling:   &sync.WaitGroup{},
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
func (c *Client) Send(ctx context.Context, topic Topic, content any) error {
	if c.isClosed {
		return errors.New("send on closed connection")
	}
	msg, err := newMessage(topic, content)
	if err != nil {
		return err
	}
	c.tx <- msg
	return nil
}

func (c *Client) readPump() {
	for {
		select {
		case <-c.fin:
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
					if err := handler.Handle(c, body); err != nil {
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
		case <-c.fin:
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
	case <-c.fin:
		return nil
	}
}
