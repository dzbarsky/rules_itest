package logger

import (
	"bytes"
	"hash/fnv"
	"io"
)

const (
	Reset = "\033[0m"

	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Purple = "\033[35m"
	Cyan   = "\033[36m"
)

var colors = []string{Red, Green, Yellow, Blue, Purple, Cyan}

func Colorize(s string) string {
	hash := fnv.New32a()
	hash.Write([]byte(s))
	return colors[hash.Sum32()%uint32(len(colors))]
}

func New(prefix string, color string, out io.Writer) io.WriteCloser {
	return &Logger{
		prefix: []byte(color + prefix + Reset),
		out:    out,
	}
}

type Logger struct {
	prefix []byte
	out    io.Writer
	buf    bytes.Buffer
}

func (l *Logger) Write(data []byte) (int, error) {
	written := 0

	lastNewline := 0
	for i, b := range data {
		if b == '\n' {
			line := append(
				append(l.prefix, l.buf.Bytes()...),
				data[lastNewline:i+1]...,
			)
			n, err := l.out.Write(line)
			written += n
			if err != nil {
				return written, err
			}

			l.buf.Reset()
			lastNewline = i + 1
		}
	}

	l.buf.Write(data[lastNewline:])
	return len(data), nil
}

func (l *Logger) Close() error {
	data := append(l.prefix, l.buf.Bytes()...)
	_, err := l.out.Write(data)
	return err
}
