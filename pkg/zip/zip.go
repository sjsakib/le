package zip

import (
	"compress/flate"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Summary struct {
	Size int64
	ETag string
}

type Archive interface {
	Read(p []byte) (int, error)
	TargetName() string
}

type archive struct {
	path           string
	r              io.Reader
	w              *CountingWriter
	startOnce      *sync.Once
	summary        *Summary
	shouldCompress bool
}

type UncompressedArchive struct {
	archive
}

type CompressedArchive struct {
	archive
}

func New(path string, shouldCompress bool) Archive {
	pr, pw := io.Pipe()

	a := archive{
		path:           path,
		r:              pr,
		w:              &CountingWriter{W: pw},
		startOnce:      &sync.Once{},
		shouldCompress: shouldCompress,
	}

	if shouldCompress {
		return &CompressedArchive{a}
	} else {
		return &UncompressedArchive{a}
	}
}

func (a *archive) Read(p []byte) (int, error) {
	a.startOnce.Do(func() {
		go func() {
			defer a.w.W.Close()
			err := a.write()
			if err != nil {
				a.w.W.CloseWithError(err)
			}
		}()
	})

	return a.r.Read(p)
}

func (a *archive) TargetName() string {
	return filepath.Base(a.path) + ".zip"
}

func (a *UncompressedArchive) Size() int64 {
	if a.summary == nil {
		err := a.readSummary()
		if err != nil {
			return -1
		}
	}

	return a.summary.Size
}

func (a *UncompressedArchive) ETag() string {
	if a.summary == nil {
		err := a.readSummary()
		if err != nil {
			return ""
		}
	}

	return fmt.Sprintf(`"%s"`, a.summary.ETag)
}

func (a *UncompressedArchive) SeekForward(offset int64) (int64, error) {
	a.w.SeekOffset += uint64(offset)

	return int64(a.w.SeekOffset), nil
}

type centralDirectoryEntry struct {
	name              string
	crc32             uint32
	compressedSize    uint64
	uncompressedSize  uint64
	localHeaderOffset uint64
	modTime           time.Time
}

func (a *archive) writeLE(num any) error {
	return binary.Write(a.w, binary.LittleEndian, num)
}

func (a *archive) write() error {
	entries := make([]centralDirectoryEntry, 0)

	dirLen := len(a.path)

	centralDirOffset := uint64(0)
	err := filepath.WalkDir(a.path, func(path string, d fs.DirEntry, err error) error {
		offset := a.w.Offset
		if d.IsDir() {
			return nil
		}

		a.writeLE(SigLocalHeader)

		a.writeLE(ZipVer45)

		// flag bit set to 3
		a.writeLE(FlagInfoComesLater | FlagUTF8Filename)

		if a.shouldCompress {
			a.writeLE(MethodDeflate)
		} else {
			a.writeLE(MethodStore)
		}

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
		a.writeLE(Max32)
		a.writeLE(Max32)

		// file name
		name := path[dirLen+1:]
		nameLen := len(name)
		a.writeLE(uint16(nameLen))
		a.writeLE(LocalExtraFieldSize) // extra field length

		_, err = io.WriteString(a.w, name)
		if err != nil {
			return err
		}

		// extra field
		a.writeLE(Zip64ExtraFieldID)
		a.writeLE(LocalZip64ExtraFieldSize)
		a.writeLE(uint64(0))
		a.writeLE(uint64(0))

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		crc := crc32.NewIEEE()

		var w io.Writer = a.w

		if a.shouldCompress {
			df, err := flate.NewWriter(a.w, flate.HuffmanOnly)
			if err != nil {
				return err
			}
			w = df
		}

		fileDataOffset := a.w.Offset

		fileSize, err := io.Copy(io.MultiWriter(crc, w), file)
		if err != nil {
			return err
		}

		if df, ok := w.(*flate.Writer); ok {
			df.Close()
		}

		compressedSize := uint64(a.w.Offset - fileDataOffset)

		a.writeLE(SigFileDescriptor)

		a.writeLE(crc.Sum32())
		a.writeLE(uint64(fileSize))
		a.writeLE(compressedSize)

		entries = append(entries, centralDirectoryEntry{
			name:              name,
			crc32:             crc.Sum32(),
			compressedSize:    compressedSize,
			uncompressedSize:  uint64(fileSize),
			localHeaderOffset: offset,
			modTime:           info.ModTime(),
		})

		return nil
	})

	if err != nil {
		return err
	}

	centralDirOffset = uint64(a.w.Offset)
	for _, e := range entries {
		err := a.writeLE(SigCDHeader)
		if err != nil {
			return err
		}

		a.writeLE(ZipVer45) // made by
		a.writeLE(ZipVer45) // required

		a.writeLE(FlagInfoComesLater | FlagUTF8Filename) // match local

		if a.shouldCompress {
			a.writeLE(MethodDeflate)
		} else {
			a.writeLE(MethodStore)
		}

		dosTime, dosDate := dosDateTime(e.modTime)
		a.writeLE(dosTime)
		a.writeLE(dosDate)

		a.writeLE(e.crc32)
		a.writeLE(Max32)
		a.writeLE(Max32)

		a.writeLE(uint16(len(e.name)))
		a.writeLE(Zip64ExtraFieldSize) // extra field length
		a.writeLE(uint16(0))

		a.writeLE(uint16(0)) // disk start
		a.writeLE(uint16(0)) // internal attrs
		a.writeLE(uint32(0)) // external attrs

		a.writeLE(Max32) // local header offset

		io.WriteString(a.w, e.name)

		// extra field
		a.writeLE(Zip64ExtraFieldID)
		a.writeLE(ExtraFieldSize)

		a.writeLE(e.uncompressedSize)
		a.writeLE(e.compressedSize)

		a.writeLE(e.localHeaderOffset)

	}

	centralDirSize := uint64(a.w.Offset - centralDirOffset)

	zip64EOCDOffset := a.w.Offset

	a.writeLE(SigZip64EOCD)
	a.writeLE(uint64(44)) // size of zip64 EOCD

	a.writeLE(ZipVer45)
	a.writeLE(ZipVer45)

	a.writeLE(uint32(0))
	a.writeLE(uint32(0))

	a.writeLE(uint64(len(entries)))
	a.writeLE(uint64(len(entries)))

	a.writeLE(uint64(centralDirSize))
	a.writeLE(uint64(centralDirOffset))

	a.writeLE(SigZip64EOCDLocator)
	a.writeLE(uint32(0)) // disk index
	a.writeLE(uint64(zip64EOCDOffset))
	a.writeLE(uint32(1)) // total disk count

	// classic EOCD
	a.writeLE(SigEOCD)
	a.writeLE(uint16(0))
	a.writeLE(uint16(0))

	a.writeLE(Max16)
	a.writeLE(Max16)

	a.writeLE(Max32)
	a.writeLE(Max32)

	a.writeLE(uint16(0))
	return nil
}

func (a *archive) readSummary() error {
	// walk dir for size and etag
	size := int64(98) // common headers
	etagH := sha256.New()
	fileCount := 0
	err := filepath.WalkDir(a.path, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		size += int64(info.Size())

		name := path[len(a.path)+1:]
		size +=
			(int64(len(name)) * 2) +
				148 // per file headers

		io.WriteString(etagH, name)
		binary.Write(etagH, binary.LittleEndian, info.ModTime().UnixNano())
		fileCount++

		return nil
	})
	if err != nil {
		return err
	}
	a.summary = &Summary{
		Size: size,
		ETag: base64.RawURLEncoding.EncodeToString(etagH.Sum(nil)[:12]),
	}
	return nil
}
