package websockets

import "log/slog"

type Logger interface {
	Infof(msg string, args ...any)
	Errorf(msg string, args ...any)
}

type nilLogger struct{}

func (n nilLogger) Infof(string, ...any) {}

func (n nilLogger) Errorf(string, ...any) {}

var _ Logger = nilLogger{}

type SlogWrapper struct {
	s *slog.Logger
}

func NewSlog(s *slog.Logger) Logger {
	return SlogWrapper{s: s}
}

func (s SlogWrapper) Infof(msg string, args ...any) {
	s.s.Info(msg, args...)
}

func (s SlogWrapper) Errorf(msg string, args ...any) {
	s.s.Error(msg, args...)
}
