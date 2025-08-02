package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.sakib.dev/le/pkg/nanoid"
	"go.sakib.dev/le/pkg/utils"
)

type fileHandler struct {
	defaultServer http.Handler
	root          http.Dir
}

func newHandler(dir string) http.Handler {
	return &fileHandler{
		defaultServer: http.FileServer(http.Dir(dir)),
		root:          http.Dir(dir),
	}
}

func (h fileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	connID := nanoid.New()
	clientIP, _, _ := net.SplitHostPort(r.RemoteAddr)

	log.Printf("[REQUEST] %s | %s - %s", connID, clientIP, r.URL.Path)

	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		log.Printf("[405] %s | %s - %s", connID, clientIP, r.URL.Path)
		return
	}

	// get root path
	absRoot, err := filepath.Abs(string(h.root))
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("[500] %s | %s - error resolving root path: %v", connID, clientIP, err)
		return
	}

	// because macOS is a special snowflake
	absRoot, err = filepath.EvalSymlinks(absRoot)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("[500] %s | %s - error resolving root symlinks: %v", connID, clientIP, err)
		return
	}

	requestedPath := filepath.Join(string(h.root), r.URL.Path)

	// absolute path of the requested file
	absPath, err := filepath.Abs(requestedPath)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("[500] %s | %s - error resolving file path: %v", connID, clientIP, err)
		return
	}

	absPath, err = filepath.EvalSymlinks(absPath)
	if err != nil && !os.IsNotExist(err) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("[500] %s | %s - error evaluating symlinks: %v", connID, clientIP, err)
		return
	}

	// clean and compare
	absRoot = filepath.Clean(absRoot)
	if err == nil {
		absPath = filepath.Clean(absPath)
	}

	if err != nil && os.IsNotExist(err) {
		parentPath := filepath.Dir(absPath)
		evalParent, err := filepath.EvalSymlinks(parentPath)
		if err == nil {
			absPath = filepath.Join(evalParent, filepath.Base(absPath))
		}
	}

	// prevent prefix matching for path traversal
	if !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) && absPath != absRoot {
		http.Error(w, "Forbidden", http.StatusForbidden)
		log.Printf("[403] %s | %s - path traversal attempt: %s", connID, clientIP, r.URL.Path)
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			log.Printf("[404] %s | %s - %s", connID, clientIP, r.URL.Path)
			return
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("[500] %s | %s - error stating %s: %v", connID, clientIP, r.URL.Path, err)
		return
	}

	if info.IsDir() {
		// check if request is coming from a browser
		acceptHeader := r.Header.Get("Accept")
		isBrowser := strings.Contains(acceptHeader, "text/html")

		if isBrowser {
			log.Printf("[200] %s | %s - serving directory listing for %s", connID, clientIP, r.URL.Path)
			h.serveDirectory(w, r, absPath)
		} else {
			log.Printf("Using default file server for %s | %s - %s", connID, clientIP, r.URL.Path)
			h.defaultServer.ServeHTTP(w, r)
		}
		return
	}

	file, err := os.Open(absPath)
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		log.Printf("[500] %s | %s - error opening %s: %v", connID, clientIP, r.URL.Path, err)
		return
	}
	defer file.Close()

	var reader io.Reader = file
	var contentLength int64 = info.Size()
	var startByte int64 = 0
	var endByte int64 = 0
	var transferStart = time.Now()

	rng := r.Header.Get("Range")
	if rng != "" {
		startByte, endByte, err = utils.ParseRangeHeader(rng, info.Size())
		if err != nil {
			http.Error(w, "Invalid Range", http.StatusRequestedRangeNotSatisfiable)
			log.Printf("[416] %s | %s - invalid range %s for %s: %v", connID, clientIP, rng, r.URL.Path, err)
			return
		}

		if _, err := file.Seek(startByte, io.SeekStart); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			log.Printf("[500] %s | %s - error seeking %s: %v", connID, clientIP, r.URL.Path, err)
			return
		}

		contentLength = endByte - startByte + 1
		reader = io.LimitReader(file, contentLength)

		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", startByte, endByte, info.Size()))
		w.WriteHeader(http.StatusPartialContent)

		log.Printf("[206] %s - %s range %d-%d", clientIP, r.URL.Path, startByte, endByte)
	} else {
		log.Printf("[200] %s - %s (%d bytes)", clientIP, r.URL.Path, info.Size())
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", contentLength))
	w.Header().Set("ETag", fmt.Sprintf(`"%x-%x"`, info.ModTime().Unix(), info.Size()))

	fileName := filepath.Base(absPath)
	var totalSent int64 = 0
	var totalMBSent float64
	buf := make([]byte, 1024*1024) // 1MB buffer
	for {
		bufferStart := time.Now()

		n, readErr := reader.Read(buf)
		if readErr != nil {
			if readErr != io.EOF {
				log.Printf("[XFER ERROR] %s | %s - %s: %v", connID, clientIP, fileName, readErr)
			}
			break
		}

		if n > 0 {
			_, writeErr := w.Write(buf[:n])
			if writeErr != nil {
				log.Printf("[XFER INTERRUPTED] %s | %s | %s after %.2fMB", connID, clientIP, fileName, totalMBSent)
				break
			}
			totalSent += int64(n)
			totalMBSent = float64(totalSent) / 1024 / 1024
			mbps := 1 / time.Since(bufferStart).Seconds()
			progress := float64(totalSent) / float64(info.Size()) * 100
			log.Printf("[XFER] %s | %s - %s: %.2fMB sent, %.2fMB/s, %.2f%%", connID, clientIP, fileName, totalMBSent, mbps, progress)
		}
	}

	log.Printf("[DONE] %s | %s - %s: %.2fMB in %s", connID, clientIP, fileName, totalMBSent, time.Since(transferStart))
}
