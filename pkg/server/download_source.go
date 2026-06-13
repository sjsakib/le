package server

import (
	"fmt"
	"io"
	"os"
)

type downloadSource interface {
	io.Reader
	TargetName() string
}

type resumableSource interface {
	downloadSource
	SeekForward(offset int64) (int64, error)
	Size() int64
	ETag() string
}

type fileSource struct {
	file     *os.File
	fileInfo os.FileInfo
}

func (s *fileSource) Read(p []byte) (n int, err error) {
	return s.file.Read(p)
}

func (s *fileSource) SeekForward(offset int64) (int64, error) {
	return s.file.Seek(offset, io.SeekCurrent)
}

func (s *fileSource) Size() int64 {
	return s.fileInfo.Size()
}

func (s *fileSource) ETag() string {
	return fmt.Sprintf(`"%x-%x"`, s.fileInfo.ModTime().Unix(), s.fileInfo.Size())
}

func (s *fileSource) TargetName() string {
	return s.fileInfo.Name()
}
