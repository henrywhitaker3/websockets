package websockets

import (
	"context"
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
