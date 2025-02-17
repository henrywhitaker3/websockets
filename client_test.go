package websockets

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestItHandlesDisconnectedConnections(t *testing.T) {
	client, server, cancel := ClientServer(t)
	defer cancel()
	require.Nil(t, client.Client.Send(context.Background(), bongo, nil))

	server.Server.Close()
	server.Http.Close()
	time.Sleep(time.Millisecond * 100)

	require.NotNil(t, client.Client.Send(context.Background(), bongo, nil))
}

func TestItReconnectsWhenItDisconnect(t *testing.T) {
	client, server, cancel := ClientServer(t)
	defer cancel()

	h := &simpleHandler{}
	require.Nil(t, server.Server.Register(bongo, h))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	require.Nil(t, client.Client.Send(ctx, bongo, nil, WithAck()))
	require.True(t, h.called)
	h.called = false
	require.False(t, h.called)

	tcpConn, ok := client.Client.conn.UnderlyingConn().(*net.TCPConn)
	require.True(t, ok)
	require.Nil(t, tcpConn.SetLinger(0))
	require.Nil(t, tcpConn.Close())

	require.Nil(t, client.Client.Send(ctx, bongo, nil, WithAck()))
	require.True(t, h.called)
}

type demo struct {
	Text string `json:"text"`
}

type structHandler struct {
	called bool
}

func (s structHandler) Empty() any {
	return &demo{}
}

func (s *structHandler) Handle(conn Connection, content any) error {
	val := content.(*demo)
	if val.Text != "apples" {
		panic("not correct")
	}
	s.called = true
	return nil
}

func TestItUnmarshalsStructs(t *testing.T) {
	client, server, cancel := ClientServer(t)
	defer cancel()

	h := &structHandler{}
	require.Nil(t, server.Server.Register(bongo, h))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	require.Nil(t, client.Client.Send(ctx, bongo, demo{Text: "apples"}, WithSuccess()))
	require.True(t, h.called)
}
