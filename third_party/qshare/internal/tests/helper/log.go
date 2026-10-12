package helper

type Logger struct {
}

func (l Logger) Error(msg string, err error, args ...any) {
	// do nothing
}

func (l Logger) Warn(msg string, args ...any) {
	// do nothing
}

func (l Logger) Info(msg string, args ...any) {
	// do nothing
}

func (l Logger) Debug(msg string, args ...any) {
	// do nothing
}
