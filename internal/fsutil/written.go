package fsutil

import (
	"io"
	"sync/atomic"
)

var written atomic.Int64

func CountWritten(n int) { written.Add(int64(n)) }

func Written() int64 { return written.Load() }

type countingWriter struct{ w io.Writer }

func (c countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	CountWritten(n)
	return n, err
}
