package websockets

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestItConnects(t *testing.T) {
	ClientServer(t)
}

const (
	bongo Topic = "bongo"
)

func TestItRoutesMessagesAsync(t *testing.T) {
	client, server, cancel := ClientServer(t)
	defer cancel()

	sh := &simpleHandler{}
	require.Nil(t, server.Server.Register(bongo, sh))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	require.Nil(t, client.Client.Send(ctx, bongo, "hello"))

	time.Sleep(time.Millisecond * 250)
	assert.True(t, sh.called, "server did not receive the message")
	server.Logger.assertErrors(t, 0)
	client.Logger.assertErrors(t, 0)
}
