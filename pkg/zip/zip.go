package zip

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

type Archive struct {
	path           string
	reader         io.Reader
	shouldCompress bool
}

func New(path string, shouldCompress bool) *Archive {
	buf := bytes.Buffer{}

	a := Archive{path: path, reader: &buf, shouldCompress: shouldCompress}

	a.write(path, &buf)
	return &a
}

func (v *Archive) Read(p []byte) (int, error) {
	return v.reader.Read(p)
}

func (v *Archive) TargetName() string {
	return filepath.Base(v.path) + ".zip"
}

type CountingWriter struct {
	W io.Writer
	N uint64
}

func (c *CountingWriter) Write(p []byte) (int, error) {
	n, err := c.W.Write(p)
	c.N += uint64(n)
	return n, err
}

type centralDirectoryEntry struct {
	name              string
	crc32             uint32
	compressedSize    uint32
	uncompressedSize  uint32
	localHeaderOffset uint32
	modTime           time.Time
}

func (v *Archive) write(dir string, w io.Writer) error {
	cw := &CountingWriter{W: w}
	entries := make([]centralDirectoryEntry, 0)

	dirLen := len(dir)

	centralDirOffset := uint64(0)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		offset := cw.N
		if d.IsDir() {
			return nil
		}

		cw.Write([]byte{'P', 'K', 0x03, 0x04})

		binary.Write(cw, binary.LittleEndian, uint16(20))

		// flag bit set to 3
		binary.Write(cw, binary.LittleEndian, uint16(0x0008))

		// compression method set to 0 (store)
		binary.Write(cw, binary.LittleEndian, uint16(0))

		info, err := d.Info()
		if err != nil {
			return err
		}

		// mod time date
		dosTime, dosDate := dosDateTime(info.ModTime())
		binary.Write(cw, binary.LittleEndian, dosTime)
		binary.Write(cw, binary.LittleEndian, dosDate)

		// crc + size uknown
		binary.Write(cw, binary.LittleEndian, uint32(0))
		binary.Write(cw, binary.LittleEndian, uint32(0))
		binary.Write(cw, binary.LittleEndian, uint32(0))

		// file name
		name := path[dirLen:]
		nameLen := len(name)
		binary.Write(cw, binary.LittleEndian, uint16(nameLen))
		binary.Write(cw, binary.LittleEndian, uint16(0))

		_, err = io.WriteString(cw, name)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		crc := crc32.NewIEEE()
		n, err := io.Copy(io.MultiWriter(crc, cw), file)
		if err != nil {
			return err
		}

		binary.Write(cw, binary.LittleEndian, uint32(0x08074b50))

		binary.Write(cw, binary.LittleEndian, crc.Sum32())
		binary.Write(cw, binary.LittleEndian, uint32(n))
		binary.Write(cw, binary.LittleEndian, uint32(n))

		entries = append(entries, centralDirectoryEntry{
			name:              path,
			crc32:             crc.Sum32(),
			compressedSize:    uint32(n),
			uncompressedSize:  uint32(n),
			localHeaderOffset: uint32(offset),
			modTime:           info.ModTime(),
		})

		return nil
	})

	if err != nil {
		return err
	}

	centralDirOffset = uint64(cw.N)
	for _, e := range entries {
		_, err := cw.Write([]byte{'P', 'K', 0x01, 0x02})
		if err != nil {
			return err
		}

		binary.Write(cw, binary.LittleEndian, uint16(20)) // v made by
		binary.Write(cw, binary.LittleEndian, uint16(20)) // v required

		binary.Write(w, binary.LittleEndian, uint16(0x0008)) // match local

		binary.Write(cw, binary.LittleEndian, uint16(0)) // stored

		dosTime, dosDate := dosDateTime(e.modTime)
		binary.Write(cw, binary.LittleEndian, dosTime)
		binary.Write(cw, binary.LittleEndian, dosDate)

		binary.Write(cw, binary.LittleEndian, e.crc32)
		binary.Write(cw, binary.LittleEndian, e.compressedSize)
		binary.Write(cw, binary.LittleEndian, e.uncompressedSize)

		binary.Write(cw, binary.LittleEndian, uint16(len(e.name)))
		binary.Write(cw, binary.LittleEndian, uint16(0))
		binary.Write(cw, binary.LittleEndian, uint16(0))

		binary.Write(cw, binary.LittleEndian, uint16(0)) // disk start
		binary.Write(cw, binary.LittleEndian, uint16(0)) // internal attrs
		binary.Write(cw, binary.LittleEndian, uint32(0)) // external attrs

		binary.Write(cw, binary.LittleEndian, e.localHeaderOffset)

		io.WriteString(cw, e.name)

	}

	binary.Write(cw, binary.LittleEndian, uint16(0))

	centralDirSize := uint64(cw.N - centralDirOffset)

	cw.Write([]byte{'P', 'K', 0x05, 0x06})

	binary.Write(cw, binary.LittleEndian, uint16(0))
	binary.Write(cw, binary.LittleEndian, uint16(0))

	binary.Write(cw, binary.LittleEndian, uint32(len(entries)))
	binary.Write(cw, binary.LittleEndian, uint32(len(entries)))

	binary.Write(cw, binary.LittleEndian, uint32(centralDirSize))
	binary.Write(cw, binary.LittleEndian, uint32(centralDirOffset))

	binary.Write(cw, binary.LittleEndian, uint16(0))
	return nil
}
