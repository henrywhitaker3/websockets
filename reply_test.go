package websockets

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServerReplies(t *testing.T) {
	client, server, cancel := ClientServer(t)
	defer cancel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	require.Nil(t, server.Server.Register(bongo, &replyHandler{
		responds: "apples",
	}))

	var out string
	require.Nil(t, client.Client.Send(ctx, bongo, "something", WithReply(&out)))
	server.Logger.assertErrors(t, 0)
	require.Equal(t, "apples", out)
}

func TestServerRepliesAndAcks(t *testing.T) {
	client, server, cancel := ClientServer(t)
	defer cancel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	require.Nil(t, server.Server.Register(bongo, &replyHandler{
		responds: "apples",
	}))

	var out string
	require.Nil(t, client.Client.Send(ctx, bongo, "something", WithAck(), WithReply(&out)))
	server.Logger.assertErrors(t, 0)
	require.Equal(t, "apples", out)
}

func TestClientReplies(t *testing.T) {
	client, server, cancel := ClientServer(t)
	defer cancel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	require.Nil(t, client.Client.Register(bongo, &replyHandler{
		responds: "apples",
	}))

	var out string
	require.Nil(t, server.Server.Once(ctx, bongo, "something", WithReply(&out)))
	server.Logger.assertErrors(t, 0)
	require.Equal(t, "apples", out)
}

func TestClientRepliesAndAcks(t *testing.T) {
	client, server, cancel := ClientServer(t)
	defer cancel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	require.Nil(t, client.Client.Register(bongo, &replyHandler{
		responds: "apples",
	}))

	var out string
	require.Nil(t, server.Server.Once(ctx, bongo, "something", WithAck(), WithReply(&out)))
	server.Logger.assertErrors(t, 0)
	require.Equal(t, "apples", out)
}
