package zip

import "io"

type CountingWriter struct {
	W          *io.PipeWriter
	Offset     uint64
	SeekOffset uint64
}

func (c *CountingWriter) Write(p []byte) (int, error) {
	var discarded uint64
	if c.SeekOffset > c.Offset {
		discarded = min(max(c.SeekOffset-c.Offset, 0), uint64(len(p)))
	}

	n, err := c.W.Write(p[discarded:])

	totalWrite := discarded + uint64(n)
	c.Offset += totalWrite
	return n + int(discarded), err
}
