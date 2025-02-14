package websockets

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestItConnects(t *testing.T) {
	server := NewServer(ServerOpts{})
	defer server.Close()
	srv := httptest.NewServer(server)
	defer srv.Close()

	client, err := NewClient(ClientOpts{
		Addr: urlToWsUrl(srv.URL),
	})
	require.Nil(t, err)
	defer client.Close()
}

const (
	bongo Topic = "bongo"
)

func TestItRoutesMessages(t *testing.T) {
	srvLog := &testLogger{}
	server := NewServer(ServerOpts{
		Logger: srvLog,
	})
	defer server.Close()
	srv := httptest.NewServer(server)
	defer srv.Close()
	sh := &simpleHandler{responds: "bingo"}
	require.Nil(t, server.Register(bongo, sh))

	clientLog := &testLogger{}
	client, err := NewClient(ClientOpts{
		Addr:   urlToWsUrl(srv.URL),
		Logger: clientLog,
	})
	require.Nil(t, err)
	defer client.Close()

	ch := &simpleHandler{responds: "bingo"}
	require.Nil(t, client.Register(bongo, ch))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	require.Nil(t, client.Send(ctx, bongo, "hello"))

	time.Sleep(time.Millisecond * 250)
	assert.True(t, sh.called, "server did not receive the message")
	time.Sleep(time.Millisecond * 250)
	assert.True(t, ch.called, "client did not receive the message")
	srvLog.assertErrors(t, 0)
	clientLog.assertErrors(t, 0)
}

type simpleHandler struct {
	called   bool
	responds string
}

func (s simpleHandler) Empty() any {
	var out string
	return out
}

func (s *simpleHandler) Handle(conn Connection, out any) error {
	s.called = true
	return conn.Send(conn.Context(), bongo, s.responds)
}

var _ Handler = &simpleHandler{}

type testLogger struct {
	infos  []string
	errors []string
}

func (t *testLogger) Infof(msg string, args ...any) {
	t.infos = append(t.infos, fmt.Sprintf(msg, args...))
}

func (t *testLogger) Errorf(msg string, args ...any) {
	t.errors = append(t.errors, fmt.Sprintf(msg, args...))
}

func (l testLogger) assertInfos(t *testing.T, count int) {
	require.Equal(
		t,
		count,
		len(l.infos),
		"expected %d infos, got %d: %s",
		count,
		len(l.infos),
		strings.Join(l.infos, ", "),
	)
}

func (l testLogger) assertErrors(t *testing.T, count int) {
	require.Equal(
		t,
		count,
		len(l.errors),
		"expected %d errors, got %d: %s",
		count,
		len(l.errors),
		strings.Join(l.errors, ", "),
	)
}

func urlToWsUrl(url string) string {
	return strings.Replace(url, "http://", "ws://", 1)
}
