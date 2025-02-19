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
	"github.com/prometheus/client_golang/prometheus"
)

type Client struct {
	conn *websocket.Conn

	control *control

	handlers   map[Topic]Handler
	handlersMu *sync.RWMutex

	handling *sync.WaitGroup
	pipes    map[string]chan *message
	pipesMu  *sync.RWMutex

	fin      chan struct{}
	closer   *sync.Once
	isClosed bool

	disconnect   chan struct{}
	reconnecting bool

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

	// The duration to wait after each connection attempt (default: 1s)
	ReconnectPause time.Duration

	// The number of times the client will try to reconnect (default: 10)
	ReconnectLimit int

	// Metrics to track client activity, if nil (or any are empty) defaults will
	// be created
	Metrics *ClientMetrics

	Logger Logger
}

func NewClient(opts ClientOpts) (*Client, error) {
	opts.Addr = strings.ReplaceAll(opts.Addr, "http://", "ws://")
	opts.Addr = strings.ReplaceAll(opts.Addr, "https://", "wss://")
	if opts.Logger == nil {
		opts.Logger = nilLogger{}
	}
	if opts.ReplyTimeout == 0 {
		opts.ReplyTimeout = time.Second
	}
	if opts.ReconnectLimit == 0 {
		opts.ReconnectLimit = 10
	}
	if opts.ReconnectPause == 0 {
		opts.ReconnectPause = time.Second
	}
	if opts.Metrics == nil {
		opts.Metrics = &ClientMetrics{}
	}
	buildClientMetrics(opts.Metrics)
	c := &Client{
		control:    newControl(),
		handlers:   map[Topic]Handler{},
		handlersMu: &sync.RWMutex{},
		handling:   &sync.WaitGroup{},
		pipes:      map[string]chan *message{},
		pipesMu:    &sync.RWMutex{},
		fin:        make(chan struct{}, 1),
		disconnect: make(chan struct{}),
		closer:     &sync.Once{},
		logger:     opts.Logger,
		opts:       opts,
	}

	c.logger.Debugw("trying to connect to the server")
	go c.watchForDisconnects()
	c.triggerReconnect()

	return c, nil
}

func (c *Client) watchForDisconnects() {
	c.logger.Debugw("watching for disconnects")
	for {
		select {
		case <-c.Closed():
			return
		case <-c.disconnect:
			c.logger.Infow("observed disconnect")
			c.reconnect()
			c.control.unlockConn()
			c.reconnecting = false
		}
	}
}

// Run the connection loop
func (c *Client) reconnect() {
	if c.isClosed {
		return
	}
	count := 1
	for {
		if count == c.opts.ReconnectLimit {
			c.logger.Errorw("client reached reconnect limit")
			c.Close()
			return
		}
		c.logger.Debugw("connecting to server", "attempt", count)
		c.opts.Metrics.Reconnections.Inc()
		if err := c.connect(); err != nil {
			c.logger.Errorw("connection failed", "error", err)
			time.Sleep(c.opts.ReconnectPause)
			count++
			continue
		}
		c.logger.Debugw("connected to server")
		go c.readPump()
		return
	}
}

func (c *Client) triggerReconnect() {
	if !c.reconnecting {
		c.control.lockConn()
		c.reconnecting = true
		c.disconnect <- struct{}{}
	}
}

func (c *Client) connect() error {
	conn, resp, err := websocket.DefaultDialer.Dial(c.opts.Addr, c.opts.Headers)
	if err != nil {
		if resp == nil {
			return fmt.Errorf("connect error: %w", err)
		}
		return fmt.Errorf("connect error [%d]: %w", resp.StatusCode, err)
	}
	c.conn = conn
	return nil
}

// Register handlers for topics originating from the server
func (c *Client) Register(topic Topic, h Handler) error {
	if slices.Contains(protectedTopics, topic) {
		return errors.New("cannot register protected topic name")
	}
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	c.handlers[topic] = h
	c.opts.Metrics.HandlersRegistered.Inc()
	return nil
}

// Sends a message to the server and unmarshalls it into output
func (c *Client) Send(ctx context.Context, topic Topic, content any, flags ...Flag) error {
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
	if c.isClosed {
		return nil, errors.New("send on closed connection")
	}
	if len(msg.Id) == 0 {
		return nil, errors.New("reply message must contain id")
	}
	pipe := make(chan *message, count)
	c.listen(msg.Id, pipe)
	defer c.forget(msg.Id)

	start := time.Now()
	defer func() {
		c.opts.Metrics.ReplyTime.WithLabelValues(string(msg.Topic)).
			Observe(time.Since(start).Seconds())
	}()

	if err := c.write(msg); err != nil {
		return nil, err
	}

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
	return c.write(msg)
}

func (c *Client) listen(id []byte, pipe chan *message) {
	c.opts.Metrics.Inflight.Inc()
	c.pipesMu.Lock()
	defer c.pipesMu.Unlock()
	c.pipes[string(id)] = pipe
}

func (c *Client) forget(id []byte) {
	c.opts.Metrics.Inflight.Add(-1)
	c.pipesMu.Lock()
	defer c.pipesMu.Unlock()
	delete(c.pipes, string(id))
}

func (c *Client) getPipe(id []byte) (chan *message, bool) {
	c.pipesMu.RLock()
	defer c.pipesMu.RUnlock()
	pipe, ok := c.pipes[string(id)]
	return pipe, ok
}

func (c *Client) readPump() {
	for {
		select {
		case <-c.Closed():
			return
		default:
			c.control.lockRx()
			var msg message
			err := c.conn.ReadJSON(&msg)
			c.control.unlockRx()
			if err != nil {
				c.opts.Metrics.ReadErrors.Inc()
				if errors.Is(err, websocket.ErrCloseSent) ||
					websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					c.logger.Infow("server closed connection")
					c.Close()
					return
				}
				c.triggerReconnect()
				c.logger.Errorw("read incoming message failed", "error", err)
				return
			}

			if slices.Contains([]Topic{ack, reply, success, errorT}, msg.Topic) {
				pipe, ok := c.getPipe(msg.Id)
				if !ok {
					c.logger.Errorw("ack or reply has no recevier", "topic", msg.Topic)
					continue
				}
				pipe <- &msg
				continue
			}

			if msg.ShouldAck {
				ctx, cancel := context.WithTimeout(context.Background(), c.opts.ReplyTimeout)
				if err := c.sendForget(ctx, &message{Id: msg.Id, Topic: ack}); err != nil {
					cancel()
					c.logger.Errorw("sending ack reply", "error", err)
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
						c.logger.Errorw("unmarshal message content", "error", err)
						return
					}
					conn, err := newClientConnection(c, &msg)
					if err != nil {
						c.logger.Errorw("could not create client connection", "error", err)
						return
					}
					if err := handler.Handle(conn, body); err != nil {
						c.logger.Errorw("handler returned error", "error", err)
					}
				}()
				continue
			}

			c.logger.Errorw("unhandled message", "topic", msg.Topic)
		}
	}
}

// Returns a new context for use in handlers
func (c *Client) Context() context.Context {
	return context.Background()
}

func (c *Client) write(msg *message) error {
	if c.isClosed {
		return errors.New("send on closed connection")
	}
	start := time.Now()

	c.control.lockTx()
	err := c.conn.WriteJSON(msg)
	c.control.unlockTx()
	if err != nil {
		c.opts.Metrics.WriteErrors.Inc()
		if errors.Is(err, websocket.ErrCloseSent) ||
			websocket.IsCloseError(err, websocket.CloseNormalClosure) {
			c.logger.Infow("server closed connection, closing...")
			c.Close()
			return err
		}
		c.triggerReconnect()
		c.logger.Errorw("sending message failed", "error", err)
		return err
	}
	c.opts.Metrics.MessagesSent.WithLabelValues(string(msg.Topic)).
		Observe(time.Since(start).Seconds())
	return nil
}

func (c *Client) Close() error {
	c.handling.Wait()
	c.close()
	return nil
}

func (c *Client) Closed() <-chan struct{} {
	return c.fin
}

func (c *Client) close() {
	c.closer.Do(func() {
		if c.conn != nil {
			c.conn.Close()
		}
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

// Register metrics against the provided registry, will ignore any errors
// returned when registering the metrics
func (c *Client) RegisterMetrics(reg prometheus.Registerer) {
	reg.Register(c.opts.Metrics.MessagesSent)
	reg.Register(c.opts.Metrics.HandlersRegistered)
	reg.Register(c.opts.Metrics.Inflight)
	reg.Register(c.opts.Metrics.ReplyTime)
	reg.Register(c.opts.Metrics.ReadErrors)
	reg.Register(c.opts.Metrics.WriteErrors)
	reg.Register(c.opts.Metrics.Reconnections)
}

// Register metrics against the provided registry, will panic is the registry errors
func (c *Client) MustRegisterMetrics(reg prometheus.Registerer) {
	reg.MustRegister(c.opts.Metrics.MessagesSent)
	reg.MustRegister(c.opts.Metrics.HandlersRegistered)
	reg.MustRegister(c.opts.Metrics.Inflight)
	reg.MustRegister(c.opts.Metrics.ReplyTime)
	reg.MustRegister(c.opts.Metrics.ReadErrors)
	reg.MustRegister(c.opts.Metrics.WriteErrors)
	reg.MustRegister(c.opts.Metrics.Reconnections)
}
