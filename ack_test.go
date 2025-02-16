package websockets

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerSendsAnAck(t *testing.T) {
	client, server, cancel := ClientServer(t)
	defer cancel()

	require.Nil(t, server.Server.Register(bongo, &simpleHandler{}))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	assert.Nil(t, client.Client.Send(ctx, bongo, nil, WithAck()))
	server.Logger.assertErrors(t, 0)
}

func TestClientTimesOutOnAck(t *testing.T) {
	client, server, cancel := ClientServer(t)
	defer cancel()

	require.Nil(t, server.Server.Register(bongo, &simpleHandler{}))

	ctx, cancel := context.WithTimeout(context.Background(), time.Microsecond)
	defer cancel()

	require.NotNil(t, client.Client.Send(ctx, bongo, nil, WithAck()))
}

func TestClientSendsAnAck(t *testing.T) {
	client, server, cancel := ClientServer(t)
	defer cancel()

	require.Nil(t, client.Client.Register(bongo, &simpleHandler{}))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	assert.Nil(t, server.Server.Once(ctx, bongo, nil, WithAck()))
	client.Logger.assertErrors(t, 0)
	server.Logger.assertErrors(t, 0)
}
