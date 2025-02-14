package websockets

type Logger interface {
	Infof(msg string, args ...any)
	Errorf(msg string, args ...any)
}

type nilLogger struct{}

func (n nilLogger) Infof(string, ...any) {}

func (n nilLogger) Errorf(string, ...any) {}

var _ Logger = nilLogger{}
