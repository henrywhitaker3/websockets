package websockets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/olahol/melody"
)

type Server struct {
	m          *melody.Melody
	handlers   map[Topic]Handler
	handlersMu *sync.RWMutex
	logger     Logger

	pipes   map[string]chan *message
	pipesMu *sync.RWMutex

	replyTimeout time.Duration
}

type ServerOpts struct {
	Logger Logger

	// Whether the server handles messages concurrently (default: false)
	ConcurrentHandling *bool

	// The max size of a message in bytes (default: 1024)
	MaxMessageSize int64

	// The length of time the server waits for a reply back
	// default to 1s
	ReplyTimeout time.Duration

	// Ping interval (default: 10s)
	PingInterval time.Duration
}

func NewServer(opts ServerOpts) *Server {
	m := melody.New()
	if opts.ConcurrentHandling == nil {
		def := false
		opts.ConcurrentHandling = &def
	}
	if opts.PingInterval == 0 {
		opts.PingInterval = time.Second * 10
	}
	m.Config.ConcurrentMessageHandling = *opts.ConcurrentHandling
	m.Config.MaxMessageSize = opts.MaxMessageSize
	m.Config.PingPeriod = opts.PingInterval
	if opts.Logger == nil {
		opts.Logger = nilLogger{}
	}
	if opts.ReplyTimeout == 0 {
		opts.ReplyTimeout = time.Second
	}
	for m.IsClosed() {
		// Wait for it to open
	}
	return &Server{
		m:            m,
		handlers:     map[Topic]Handler{},
		handlersMu:   &sync.RWMutex{},
		logger:       opts.Logger,
		pipes:        map[string]chan *message{},
		pipesMu:      &sync.RWMutex{},
		replyTimeout: opts.ReplyTimeout,
	}
}

func (s *Server) Register(topic Topic, handler Handler) error {
	if slices.Contains(protectedTopics, topic) {
		return errors.New("protected topic")
	}
	s.handlersMu.Lock()
	defer s.handlersMu.Unlock()
	s.handlers[topic] = handler
	return nil
}

func (s *Server) listen(id []byte, pipe chan *message) {
	s.pipesMu.Lock()
	defer s.pipesMu.Unlock()
	s.pipes[string(id)] = pipe
}

func (s *Server) forget(id []byte) {
	s.pipesMu.Lock()
	defer s.pipesMu.Unlock()
	delete(s.pipes, string(id))
}

func (s *Server) getPipe(id []byte) (chan *message, bool) {
	s.pipesMu.RLock()
	defer s.pipesMu.RUnlock()
	pipe, ok := s.pipes[string(id)]
	return pipe, ok
}

func (s *Server) handleIncoming(sess *melody.Session, data []byte) {
	var msg message
	if err := json.Unmarshal(data, &msg); err != nil {
		s.logger.Errorw("unmarhsal incoming message", "error", err)
		return
	}
	s.logger.Debugw("received message", "topic", msg.Topic)

	if msg.ShouldAck {
		ack, err := json.Marshal(message{
			Id:    msg.Id,
			Topic: ack,
		})
		if err != nil {
			s.logger.Errorw("marshal ack message", "error", err)
			return
		}
		if err := sess.Write(ack); err != nil {
			s.logger.Errorw("send ack", "error", err)
			return
		}
	}

	if slices.Contains([]Topic{ack, reply, errorT, success}, msg.Topic) {
		pipe, ok := s.getPipe(msg.Id)
		if !ok {
			s.logger.Errorw("no registered pipe for message", "topic", msg.Topic)
			return
		}
		pipe <- &msg
		return
	}

	s.handlersMu.RLock()
	handler, ok := s.handlers[msg.Topic]
	s.handlersMu.RUnlock()

	if !ok {
		s.logger.Errorw("no handler for topic", "topic", msg.Topic)
		return
	}

	body := handler.Empty()
	if err := json.Unmarshal(msg.Content, &body); err != nil {
		s.logger.Errorw("unmarhsal message content", "error", err)
		return
	}

	conn := &serverConnection{
		s:     sess,
		id:    msg.Id,
		topic: msg.Topic,
	}
	if err := handler.Handle(conn, body); err != nil {
		s.logger.Errorw("handler returned error", "error", err)
		if msg.ShouldSucceed {
			fail, err := newMessage(errorT, err.Error())
			if err != nil {
				s.logger.Errorw("generate error message", "error", err)
				return
			}
			fail.Id = msg.Id
			by, err := json.Marshal(fail)
			if err != nil {
				s.logger.Errorw("marshal error message", "error", err)
			}
			if err := sess.Write(by); err != nil {
				s.logger.Errorw("send error message", "error", err)
			}
		}
		return
	}

	if msg.ShouldSucceed {
		succ, err := json.Marshal(message{
			Id:    msg.Id,
			Topic: success,
		})
		if err != nil {
			s.logger.Errorw("marshal success message", "error", err)
			return
		}
		if err := sess.Write(succ); err != nil {
			s.logger.Errorw("send success message", "error", err)
			return
		}
	}
}

func (s *Server) Handle(w http.ResponseWriter, r *http.Request) error {
	s.m.HandleMessage(s.handleIncoming)
	return s.m.HandleRequest(w, r)
}

// Sends a message to all connected sessions
func (s *Server) Broadcast(ctx context.Context, topic Topic, content any) error {
	msg, err := newMessage(topic, content)
	if err != nil {
		return fmt.Errorf("create message: %w", err)
	}
	by, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	return s.m.Broadcast(by)
}

// Sends a message to one random connected session
func (s *Server) Once(ctx context.Context, topic Topic, content any, flags ...Flag) error {
	sessions, err := s.m.Sessions()
	if err != nil {
		return fmt.Errorf("get sessions: %w", err)
	}
	if len(sessions) < 1 {
		return errors.New("no connected sessions")
	}
	session := sessions[0]
	if len(sessions) > 1 {
		session = sessions[rand.Intn(len(sessions)-1)]
	}
	msg, err := newMessage(topic, content)
	if err != nil {
		return fmt.Errorf("create message: %w", err)
	}
	for _, flag := range flags {
		if err := flag.ModifyMessage(msg); err != nil {
			return fmt.Errorf("apply flag to message: %w", err)
		}
	}
	return s.send(session, msg)
}

func (s *Server) send(sess *melody.Session, msg *message) error {
	by, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	if err := sess.Write(by); err != nil {
		return fmt.Errorf("send message: %w", err)
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
	if count == 0 {
		return nil
	}

	pipe := make(chan *message, 1)
	s.listen(msg.Id, pipe)
	defer s.forget(msg.Id)
	messages := []*message{}

	for range count {
		ctx, cancel := context.WithTimeout(sess.Request.Context(), s.replyTimeout)
		defer cancel()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-pipe:
			messages = append(messages, msg)
		}
	}

	if len(messages) != count {
		return fmt.Errorf("expected %d replies, got %d", count, len(messages))
	}

	for i, reply := range messages {
		if i == 0 && msg.ShouldAck {
			if reply.Topic != ack {
				return fmt.Errorf("ecpected ack, got %s", msg.Topic)
			}
			continue
		}
		if i == len(messages)-1 && msg.ShouldSucceed {
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
			return fmt.Errorf("unmarshal reply into target: %w", err)
		}
	}
	return nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := s.Handle(w, r); err != nil {
		s.logger.Errorw("serve http", "error", err)
	}
}

func (s *Server) Close() error {
	return s.m.Close()
}

var _ http.Handler = &Server{}
