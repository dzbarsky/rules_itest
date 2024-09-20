package logger

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"log"
	"strconv"
)

const Reset = "\033[0m"

// Inspired by https://github.com/debug-js/debug/blob/f66cb2d9f729e1a592e72d3698e3b75329d75a25/src/node.js#L35
var colors = []int{
	20,
	21,
	26,
	27,
	32,
	33,
	38,
	39,
	40,
	41,
	42,
	43,
	44,
	45,
	56,
	57,
	62,
	63,
	68,
	69,
	74,
	75,
	76,
	77,
	78,
	79,
	80,
	81,
	92,
	93,
	98,
	99,
	112,
	113,
	128,
	129,
	134,
	135,
	148,
	149,
	160,
	161,
	162,
	163,
	164,
	165,
	166,
	167,
	168,
	169,
	170,
	171,
	172,
	173,
	178,
	179,
	184,
	185,
	196,
	197,
	198,
	199,
	200,
	201,
	202,
	203,
	204,
	205,
	206,
	207,
	208,
	209,
	214,
	215,
	220,
	221,
}

func Colorize(s string) string {
	hash := sha256.New()
	hash.Write([]byte(s))
	// Inspired by https://github.com/debug-js/debug/blob/f66cb2d9f729e1a592e72d3698e3b75329d75a25/src/node.js#L172-L173
	hashedUint32 := binary.BigEndian.Uint32(hash.Sum(nil)[:4])
	chosen := colors[hashedUint32%uint32(len(colors))]
	return "\u001B[38;5;" + strconv.Itoa(chosen) + ";1m"
}

func New(prefix string, color string, out io.Writer) io.WriteCloser {
	return &Logger{
		out: log.New(out, color+prefix+Reset, log.Ltime|log.Lmicroseconds|log.Lmsgprefix),
	}
}

type Logger struct {
	out *log.Logger
	buf bytes.Buffer
}

func (l *Logger) Write(data []byte) (int, error) {
	written := 0

	lastNewline := 0
	for i, b := range data {
		if b == '\n' {
			line := append(
				l.buf.Bytes(),
				data[lastNewline:i+1]...,
			)
			l.out.Print(string(line))
			written += len(line)
			l.buf.Reset()
			lastNewline = i + 1
		}
	}

	l.buf.Write(data[lastNewline:])
	return len(data), nil
}

func (l *Logger) Close() error {
	l.out.Print(l.buf.String())
	return nil
}
