package websockets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	srvLog := newTestLogger(t, "server")
	server := NewServer(ServerOpts{Logger: srvLog})
	srv := httptest.NewServer(server)

	clientLog := newTestLogger(t, "client")
	client, err := NewClient(ClientOpts{
		Addr:         urlToWsUrl(srv.URL),
		Logger:       clientLog,
		ReplyTimeout: time.Second * 100,
	})
	require.Nil(t, err)
	// Give it time to connect
	time.Sleep(time.Millisecond * 50)

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

type errorHandler struct{}

func (s errorHandler) Empty() any {
	return ""
}

func (s errorHandler) Handle(conn Connection, content any) error {
	return errors.New("it failed")
}

var _ Handler = errorHandler{}

type testLogger struct {
	slog   *slog.Logger
	unit   string
	infos  []string
	errors []string
	debugs []string
}

func newTestLogger(t *testing.T, unit string) *testLogger {
	t.Helper()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	t.Cleanup(func() {
		t.Log(buf.String())
	})
	return &testLogger{
		slog: logger,
		unit: unit,
	}
}

func (t *testLogger) Infow(msg string, args ...any) {
	t.infos = append(t.infos, fmt.Sprintf(msg, args...))
	args = append(args, "unit", t.unit)
	t.slog.Info(msg, args...)
}

func (t *testLogger) Errorw(msg string, args ...any) {
	t.errors = append(t.errors, fmt.Sprintf(msg, args...))
	args = append(args, "unit", t.unit)
	t.slog.Error(msg, args...)
}

func (t *testLogger) Debugw(msg string, args ...any) {
	t.debugs = append(t.debugs, fmt.Sprintf(msg, args...))
	args = append(args, "unit", t.unit)
	t.slog.Debug(msg, args...)
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
