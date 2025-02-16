package websockets

type Flag interface {
	ModifyMessage(*message) error
}

type withAck struct{}

// Tells the server to send an ack back when it receives the message
func WithAck() Flag {
	return withAck{}
}

func (w withAck) ModifyMessage(msg *message) error {
	newMsg, err := toAckedMessage(msg)
	if err != nil {
		return err
	}
	*msg = *newMsg
	return nil
}

type withReply struct {
	target any
}

func (w withReply) ModifyMessage(msg *message) error {
	newMsg, err := toRepliedMessage(msg)
	if err != nil {
		return err
	}
	newMsg, err = toSuccessfulMessage(newMsg)
	if err != nil {
		return err
	}
	newMsg.replyTarget = w.target
	*msg = *newMsg
	return nil
}

// Passes a target to the send function to unmarshal the response from the server into
func WithReply(target any) Flag {
	return &withReply{target: target}
}

type withSuccess struct{}

func (w withSuccess) ModifyMessage(msg *message) error {
	newMsg, err := toSuccessfulMessage(msg)
	if err != nil {
		return err
	}
	*msg = *newMsg
	return nil
}

// Requires the recipient to return a success message when all handlers return with no error
func WithSuccess() Flag {
	return withSuccess{}
}

type forceTopic struct {
	topic Topic
}

func (f forceTopic) ModifyMessage(msg *message) error {
	msg.Topic = f.topic
	return nil
}

func iForceTopic(t Topic) Flag {
	return forceTopic{topic: t}
}

type forceId struct {
	id []byte
}

func (f forceId) ModifyMessage(msg *message) error {
	msg.Id = f.id
	return nil
}

func iForceId(id []byte) Flag {
	return &forceId{id: id}
}
