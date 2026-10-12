package qshare

type Logger interface {
	Error(msg string, err error, args ...any)
	Warn(msg string, args ...any)
	Info(msg string, args ...any)
	Debug(msg string, args ...any)
}
