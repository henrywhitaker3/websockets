package websockets

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
)

type Topic string

const (
	ack     Topic = "ack"
	reply   Topic = "reply"
	success Topic = "success"
	errorT  Topic = "error"
)

var (
	protectedTopics = []Topic{ack, reply, success, errorT}
)

type message struct {
	Id      []byte `json:"id"`
	Topic   Topic  `json:"topic"`
	Content []byte `json:"content"`

	ShouldAck     bool `json:"should_ack"`
	ShouldReply   bool `json:"should_reply"`
	ShouldSucceed bool `json:"should_succeed"`

	replyTarget any `json:"-"`
}

func newMessage(topic Topic, content any) (*message, error) {
	bytes, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("marshal content to json: %w", err)
	}
	return &message{
		Topic:   topic,
		Content: bytes,
	}, nil
}

func toAckedMessage(msg *message) (*message, error) {
	if len(msg.Id) == 0 {
		id := make([]byte, 10)
		if _, err := rand.Read(id); err != nil {
			return nil, fmt.Errorf("generate message id: %w", err)
		}
		msg.Id = id
	}
	msg.ShouldAck = true
	return msg, nil
}

func toRepliedMessage(msg *message) (*message, error) {
	if len(msg.Id) == 0 {
		id := make([]byte, 10)
		if _, err := rand.Read(id); err != nil {
			return nil, fmt.Errorf("generate message id: %w", err)
		}
		msg.Id = id
	}
	msg.ShouldReply = true
	return msg, nil
}

func toSuccessfulMessage(msg *message) (*message, error) {
	if len(msg.Id) == 0 {
		id := make([]byte, 10)
		if _, err := rand.Read(id); err != nil {
			return nil, fmt.Errorf("generate message id: %w", err)
		}
		msg.Id = id
	}
	msg.ShouldSucceed = true
	return msg, nil
}
