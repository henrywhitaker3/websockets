package websockets

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type TestServer struct {
	Http   *httptest.Server
	Server *Server
	Logger *testLogger
}

type TestClient struct {
	Client *Client
	Logger *testLogger
}

func ClientServer(t *testing.T) (*TestClient, *TestServer, context.CancelFunc) {
	srvLog := &testLogger{unit: "server"}
	server := NewServer(ServerOpts{Logger: srvLog})
	srv := httptest.NewServer(server)

	clientLog := &testLogger{unit: "client"}
	client, err := NewClient(ClientOpts{
		Addr:         urlToWsUrl(srv.URL),
		Logger:       clientLog,
		ReplyTimeout: time.Second * 100,
	})
	require.Nil(t, err)

	return &TestClient{
			Client: client,
			Logger: clientLog,
		}, &TestServer{
			Http:   srv,
			Server: server,
			Logger: srvLog,
		}, func() {
			client.Close()
			srv.Close()
			server.Close()
		}
}

type replyHandler struct {
	called   bool
	responds string
}

func (s replyHandler) Empty() any {
	var out string
	return out
}

func (s *replyHandler) Handle(conn Connection, out any) error {
	s.called = true
	return conn.Send(conn.Context(), bongo, s.responds)
}

var _ Handler = &replyHandler{}

type simpleHandler struct {
	called bool
}

func (s simpleHandler) Empty() any {
	var out string
	return out
}

func (s *simpleHandler) Handle(conn Connection, out any) error {
	s.called = true
	return nil
}

var _ Handler = &simpleHandler{}

type testLogger struct {
	unit   string
	infos  []string
	errors []string
	debugs []string
}

func (t *testLogger) Infof(msg string, args ...any) {
	t.infos = append(t.infos, fmt.Sprintf(msg, args...))
}

func (t *testLogger) Errorf(msg string, args ...any) {
	t.errors = append(t.errors, fmt.Sprintf(msg, args...))
}

func (t *testLogger) Debugf(msg string, args ...any) {
	t.debugs = append(t.debugs, fmt.Sprintf(msg, args...))
}

func (l testLogger) assertInfos(t *testing.T, count int) {
	require.Equal(
		t,
		count,
		len(l.infos),
		"[%s] expected %d infos, got %d: %s",
		l.unit,
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
		"[%s] expected %d errors, got %d: %s",
		l.unit,
		count,
		len(l.errors),
		strings.Join(l.errors, ", "),
	)
}

func (l testLogger) assertDebugs(t *testing.T, count int) {
	require.Equal(
		t,
		count,
		len(l.debugs),
		"[%s] expected %d debugs, got %d: %s",
		l.unit,
		count,
		len(l.debugs),
		strings.Join(l.debugs, ", "),
	)
}

func urlToWsUrl(url string) string {
	return strings.Replace(url, "http://", "ws://", 1)
}
