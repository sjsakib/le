package zip

import (
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
	r              io.Reader
	w              *CountingWriter
	shouldCompress bool
}

func New(path string, shouldCompress bool) *Archive {
	pr, pw := io.Pipe()

	a := Archive{
		path:           path,
		r:              pr,
		w:              &CountingWriter{W: pw, N: 0},
		shouldCompress: shouldCompress,
	}

	go func() {
		defer pw.Close()
		err := a.write()
		if err != nil {
			pw.CloseWithError(err)
		}
	}()

	return &a
}

func (a *Archive) Read(p []byte) (int, error) {
	return a.r.Read(p)
}

func (a *Archive) TargetName() string {
	return filepath.Base(a.path) + ".zip"
}

type centralDirectoryEntry struct {
	name              string
	crc32             uint32
	compressedSize    uint32
	uncompressedSize  uint32
	localHeaderOffset uint32
	modTime           time.Time
}

func (a *Archive) writeLE(num any) error {
	return binary.Write(a.w, binary.LittleEndian, num)
}

func (a *Archive) write() error {
	entries := make([]centralDirectoryEntry, 0)

	dirLen := len(a.path)

	centralDirOffset := uint64(0)
	err := filepath.WalkDir(a.path, func(path string, d fs.DirEntry, err error) error {
		offset := a.w.N
		if d.IsDir() {
			return nil
		}

		a.writeLE(SigLocalHeader)

		a.writeLE(VerMadeBy)

		// flag bit set to 3
		a.writeLE(FlagInfoComesLater)

		// compression method set to 0 (store)
		a.writeLE(MethodStore)

		info, err := d.Info()
		if err != nil {
			return err
		}

		// mod time date
		dosTime, dosDate := dosDateTime(info.ModTime())
		a.writeLE(dosTime)
		a.writeLE(dosDate)

		// crc + size uknown
		a.writeLE(uint32(0))
		a.writeLE(uint32(0))
		a.writeLE(uint32(0))

		// file name
		name := path[dirLen:]
		nameLen := len(name)
		a.writeLE(uint16(nameLen))
		a.writeLE(uint16(0))

		_, err = io.WriteString(a.w, name)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		crc := crc32.NewIEEE()
		fileSize, err := io.Copy(io.MultiWriter(crc, a.w), file)
		if err != nil {
			return err
		}

		a.writeLE(SigFileDescriptor)

		a.writeLE(crc.Sum32())
		a.writeLE(uint32(fileSize))
		a.writeLE(uint32(fileSize))

		entries = append(entries, centralDirectoryEntry{
			name:              name,
			crc32:             crc.Sum32(),
			compressedSize:    uint32(fileSize),
			uncompressedSize:  uint32(fileSize),
			localHeaderOffset: uint32(offset),
			modTime:           info.ModTime(),
		})

		return nil
	})

	if err != nil {
		return err
	}

	centralDirOffset = uint64(a.w.N)
	for _, e := range entries {
		err := a.writeLE(SigCDHeader)
		if err != nil {
			return err
		}

		a.writeLE(VerMadeBy)
		a.writeLE(VerRequired)

		a.writeLE(FlagInfoComesLater) // match local

		a.writeLE(MethodStore)

		dosTime, dosDate := dosDateTime(e.modTime)
		a.writeLE(dosTime)
		a.writeLE(dosDate)

		a.writeLE(e.crc32)
		a.writeLE(e.compressedSize)
		a.writeLE(e.uncompressedSize)

		a.writeLE(uint16(len(e.name)))
		a.writeLE(uint16(0))
		a.writeLE(uint16(0))

		a.writeLE(uint16(0)) // disk start
		a.writeLE(uint16(0)) // internal attrs
		a.writeLE(uint32(0)) // external attrs

		a.writeLE(e.localHeaderOffset)

		io.WriteString(a.w, e.name)

	}

	centralDirSize := uint64(a.w.N - centralDirOffset)

	a.writeLE(SigEOCD)

	a.writeLE(uint16(0))
	a.writeLE(uint16(0))

	a.writeLE(uint16(len(entries)))
	a.writeLE(uint16(len(entries)))

	a.writeLE(uint32(centralDirSize))
	a.writeLE(uint32(centralDirOffset))

	a.writeLE(uint16(0))
	return nil
}
