package websockets

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestItHandlesDisconnectedConnections(t *testing.T) {
	logger := &testLogger{}
	server := NewServer(ServerOpts{Logger: logger})
	defer server.Close()
	srv := httptest.NewServer(server)
	defer srv.Close()
	require.Nil(t, server.Register(bongo, &simpleHandler{}))

	client, err := NewClient(ClientOpts{
		Addr:   urlToWsUrl(srv.URL),
		Logger: logger,
	})
	require.Nil(t, err)
	require.Nil(t, client.Send(context.Background(), bongo, nil))

	server.Close()
	srv.Close()

	require.NotNil(t, client.Send(context.Background(), bongo, nil))
}
