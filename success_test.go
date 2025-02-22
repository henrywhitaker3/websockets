package websockets

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServerReturnsSuccess(t *testing.T) {
	client, server, cancel := ClientServer(t)
	defer cancel()

	require.Nil(t, server.Server.Register(bongo, &replyHandler{responds: "bongo"}))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	var out string
	require.Nil(t, client.Client.Send(ctx, bongo, "something", WithReply(&out), WithSuccess()))
	require.Equal(t, "bongo", out)
	client.Logger.assertErrors(t, 0)
	server.Logger.assertErrors(t, 0)
}

func TestServerReturnsErrors(t *testing.T) {
	client, server, cancel := ClientServer(t)
	defer cancel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	require.Nil(t, server.Server.Register(bongo, errorHandler{}))

	err := client.Client.Send(ctx, bongo, nil, WithSuccess())
	require.NotNil(t, err)
	require.Equal(t, "it failed", err.Error())
}

func TestClientReturnsErrors(t *testing.T) {
	client, server, cancel := ClientServer(t)
	defer cancel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	require.Nil(t, client.Client.Register(bongo, errorHandler{}))

	err := server.Server.Once(ctx, bongo, nil, WithSuccess())
	require.NotNil(t, err)
	require.Equal(t, "it failed", err.Error())
}

func TestClientReturnsSuccess(t *testing.T) {
	client, server, cancel := ClientServer(t)
	defer cancel()

	require.Nil(t, client.Client.Register(bongo, &replyHandler{responds: "bongo"}))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	var out string
	require.Nil(t, server.Server.Once(ctx, bongo, "something", WithReply(&out), WithSuccess()))
	require.Equal(t, "bongo", out)
	client.Logger.assertErrors(t, 0)
	server.Logger.assertErrors(t, 0)
}
