package websockets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync"

	"github.com/olahol/melody"
)

type Server struct {
	m          *melody.Melody
	handlers   map[Topic]Handler
	handlersMu *sync.RWMutex
	logger     Logger
}

type ServerOpts struct {
	Logger Logger
}

func NewServer(opts ServerOpts) *Server {
	m := melody.New()
	if opts.Logger == nil {
		opts.Logger = nilLogger{}
	}
	return &Server{
		m:          m,
		handlers:   map[Topic]Handler{},
		handlersMu: &sync.RWMutex{},
		logger:     opts.Logger,
	}
}

func (s *Server) Register(topic Topic, handler Handler) error {
	if slices.Contains(protectedTopics, topic) {
		return errors.New("cannot register protected topic name")
	}
	s.handlersMu.Lock()
	defer s.handlersMu.Unlock()
	s.handlers[topic] = handler
	return nil
}

func (s *Server) handleIncoming(sess *melody.Session, data []byte) {
	var msg message
	if err := json.Unmarshal(data, &msg); err != nil {
		s.logger.Errorf("unmarhsal incoming message: %v", err)
		return
	}

	s.handlersMu.RLock()
	handler, ok := s.handlers[msg.Topic]
	s.handlersMu.RUnlock()

	if !ok {
		s.logger.Errorf("no handler for topic %s", msg.Topic)
		return
	}

	body := handler.Empty()
	if err := json.Unmarshal(msg.Content, &body); err != nil {
		s.logger.Errorf("unmarhsal message content: %v", err)
		return
	}

	conn := &serverConnection{
		s:     sess,
		topic: msg.Topic,
	}
	if err := handler.Handle(conn, body); err != nil {
		s.logger.Errorf("handler returned error: %v", err)
		return
	}
}

func (s *Server) Handle(w http.ResponseWriter, r *http.Request) error {
	s.m.HandleMessage(s.handleIncoming)
	return s.m.HandleRequest(w, r)
}

func (s *Server) Broadcast(ctx context.Context, topic Topic, content string) error {
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

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.Handle(w, r)
}

func (s *Server) Close() error {
	return s.m.Close()
}

var _ http.Handler = &Server{}
