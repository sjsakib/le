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
	io.Seeker
	Size() int64
	Etag() string
}

type fileSource struct {
	file     *os.File
	fileInfo os.FileInfo
}

func (s *fileSource) Read(p []byte) (n int, err error) {
	return s.file.Read(p)
}

func (s *fileSource) Seek(offset int64, whence int) (int64, error) {
	return s.file.Seek(offset, whence)
}

func (s *fileSource) Close() error {
	return s.file.Close()
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
