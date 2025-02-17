package websockets

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/olahol/melody"
)

type Connection interface {
	Context() context.Context
	Send(topic Topic, content any, flags ...Flag) error
}

type serverConnection struct {
	s     *melody.Session
	id    []byte
	topic Topic
}

func (s *serverConnection) Context() context.Context {
	return s.s.Request.Context()
}

func (s *serverConnection) Send(
	topic Topic,
	content any,
	flags ...Flag,
) error {
	var by []byte
	var err error
	if content != nil {
		by, err = json.Marshal(content)
	}
	if err != nil {
		return fmt.Errorf("marshal content: %w", err)
	}
	msg := message{
		Id:      s.id,
		Topic:   reply,
		Content: by,
	}
	out, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	return s.s.Write(out)
}

var _ Connection = &serverConnection{}

type clientConnection struct {
	client *Client
	msg    *message
}

func newClientConnection(c *Client, msg *message) (*clientConnection, error) {
	return &clientConnection{
		client: c,
		msg:    msg,
	}, nil
}

func (c *clientConnection) Send(
	topic Topic,
	content any,
	flags ...Flag,
) error {
	flags = append(flags, iForceTopic(reply), iForceId(c.msg.Id))
	return c.client.Send(c.Context(), reply, content, flags...)
}

func (c *clientConnection) Context() context.Context {
	return context.Background()
}

var _ Connection = &clientConnection{}

type Handler interface {
	// This function returns a new object to unmarshal
	// the message contents into
	Empty() any

	// This function takes in the unmarshalled data
	// and processes it
	Handle(Connection, any) error
}
