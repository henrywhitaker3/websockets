package websockets

import (
	"encoding/json"
	"fmt"
)

type Topic string

const (
	// Protected topic, used to acknowledge receipt of the message
	ack   Topic = "ack"
	done  Topic = "done"
	reply Topic = "reply"
)

var (
	protectedTopics = []Topic{ack, done}
)

type message struct {
	Topic   Topic  `json:"topic"`
	Content []byte `json:"content"`
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
