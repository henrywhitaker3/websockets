package websockets

type Flag interface {
	ModifyMessage(*message) error
}

type withAck struct{}

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
	newMsg.replyTarget = w.target
	*msg = *newMsg
	return nil
}

func WithReply(target any) Flag {
	return &withReply{target: target}
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
