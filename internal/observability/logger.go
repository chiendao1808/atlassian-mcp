package observability

import (
	"fmt"
	"io"
)

type Logger struct {
	out io.Writer
}

func NewLogger(out io.Writer) Logger {
	return Logger{out: out}
}

func (l Logger) Warnf(format string, args ...any) {
	if l.out == nil {
		return
	}
	fmt.Fprintf(l.out, format+"\n", args...)
}
