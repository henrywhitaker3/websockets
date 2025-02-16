package websockets

import "log/slog"

type Logger interface {
	Infow(msg string, args ...any)
	Errorw(msg string, args ...any)
	Debugw(msg string, args ...any)
}

type nilLogger struct{}

func (n nilLogger) Infow(string, ...any) {}

func (n nilLogger) Errorw(string, ...any) {}

func (n nilLogger) Debugw(string, ...any) {}

var _ Logger = nilLogger{}

type SlogWrapper struct {
	s *slog.Logger
}

func NewSlog(s *slog.Logger) Logger {
	return SlogWrapper{s: s}
}

func (s SlogWrapper) Infow(msg string, args ...any) {
	s.s.Info(msg, args...)
}

func (s SlogWrapper) Errorw(msg string, args ...any) {
	s.s.Error(msg, args...)
}

func (s SlogWrapper) Debugw(msg string, args ...any) {
	s.s.Debug(msg, args...)
}
