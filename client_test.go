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

type demo struct {
	Text string `json:"text"`
}

type structHandler struct{}

func (s structHandler) Empty() any {
	return demo{}
}

func (s structHandler) Handle(conn Connection, content any) error {
	val := content.(demo)
	if val.Text != "appled" {
		panic("not correct")
	}
	return nil
}

func TestItUnmarshalsStructs(t *testing.T) {
	client, server, cancel := ClientServer(t)
	defer cancel()

	require.Nil(t, server.Server.Register(bongo, structHandler{}))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	require.Nil(t, client.Client.Send(ctx, bongo, demo{Text: "apples"}))
}
