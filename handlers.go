package websockets

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/olahol/melody"
)

type Connection interface {
	Context() context.Context
	Send(ctx context.Context, topic Topic, content any) error
}

type serverConnection struct {
	s     *melody.Session
	topic Topic
}

func (s *serverConnection) Context() context.Context {
	return s.s.Request.Context()
}

func (s *serverConnection) Send(ctx context.Context, topic Topic, content any) error {
	var by []byte
	var err error
	if content != nil {
		by, err = json.Marshal(content)
	}
	if err != nil {
		return fmt.Errorf("marshal content: %w", err)
	}
	msg := message{
		Topic:   s.topic,
		Content: by,
	}
	out, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	return s.s.Write(out)
}

var _ Connection = &serverConnection{}

type Handler interface {
	// This function returns a new object to unmarshal
	// the message contents into
	Empty() any

	// This function takes in the unmarshalled data
	// and processes it
	Handle(Connection, any) error
}
