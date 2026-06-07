package zip

import "io"

type CountingWriter struct {
	W io.Writer
	N uint64
}

func (c *CountingWriter) Write(p []byte) (int, error) {
	n, err := c.W.Write(p)
	c.N += uint64(n)
	return n, err
}
