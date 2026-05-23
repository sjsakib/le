package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"go.sakib.dev/le/logger"
	"go.sakib.dev/le/pkg/nanoid"
	"go.sakib.dev/le/pkg/utils"
)

const downloadProgressLogInterval = 500 * time.Millisecond // Log download progress every 500 milliseconds

type handler struct {
	defaultServer http.Handler
	root          http.Dir
	ch            chan<- ServerEvent
}

func newHandler(dir string, ch chan<- ServerEvent) http.Handler {
	return &handler{
		defaultServer: http.FileServer(http.Dir(dir)),
		root:          http.Dir(dir),
		ch:            ch,
	}
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reqHelper := newReqHelper(w, r, h.ch)

	reqHelper.attachReqId()

	clientIP, err := utils.GetClientIP(r)
	if err != nil {
		slog.Warn("Failed to get client IP", "error", err)
		clientIP = "unknown"
	}

	clientHost, err := utils.GetClientHostname(r)
	if err != nil {
		slog.Warn("Failed to get client hostname", "error", err)
		clientHost = "unknown"
	}

	slog.InfoContext(reqHelper.ctx, "REQUEST",
		"clientIP", clientIP,
		"clientHost", clientHost,
		"userAgent", r.UserAgent(),
		"method", r.Method,
		"path", r.URL.Path)

	defer reqHelper.publishConnClose()

	if r.Method != http.MethodGet {
		reqHelper.error("Method Not Allowed", nil, http.StatusMethodNotAllowed)
		return
	}

	absPath, err := utils.SecureJoin(string(h.root), r.URL.Path)

	slog.Debug("Secure Join", "path", absPath, "root", string(h.root), "path", r.URL.Path, "error", err)

	if errors.Is(err, utils.ErrForbiddenPath) {
		reqHelper.error("FORBIDDEN", err, http.StatusForbidden)
		return
	} else if err != nil {
		reqHelper.internalServerError(err)
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			reqHelper.error("NOT FOUND", err, http.StatusNotFound)
			return
		}
		reqHelper.internalServerError(err)
		return
	}

	if info.IsDir() {
		// check if request is coming from a browser
		acceptHeader := r.Header.Get("Accept")
		isBrowser := strings.Contains(acceptHeader, "text/html")

		if isBrowser {
			slog.InfoContext(reqHelper.ctx, "OK - Serving directory with pretty UI", "path", r.URL.Path)
			h.serveDirectory(w, r, absPath)
		} else {
			slog.InfoContext(reqHelper.ctx, "OK - Serving directory default file server", "path", r.URL.Path)
			h.defaultServer.ServeHTTP(w, r)
		}
		return
	}

	file, err := os.Open(absPath)
	if err != nil {
		reqHelper.internalServerError(err)
		return
	}
	defer file.Close()
	var transferStart = time.Now()

	startByte, contentLength, reader, err := reqHelper.handleRange(file, info)
	if err != nil {
		if errors.Is(err, ErrInvalidRangeHeader) {
			reqHelper.error("Invalid Range", err, http.StatusRequestedRangeNotSatisfiable)
			return
		}
		reqHelper.internalServerError(err)
		return

	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", contentLength))
	w.Header().Set("ETag", fmt.Sprintf(`"%x-%x"`, info.ModTime().Unix(), info.Size()))

	fileDisplayPath := utils.ReplaceHome(absPath)
	var totalSent int64 = 0
	var totalMBSent float64
	buf := make([]byte, 1024*1024) // 1MB buffer
	var lastReportedSent int64 = 0
	var lastReportedTime = time.Now()
	reqHelper.publishDownloadStart(fileDisplayPath, info.Size(), startByte, startByte+contentLength-1, clientIP, clientHost)
	slog.Info("Starting download", "file", fileDisplayPath, "totalSize", info.Size(), "rangeStart", startByte, "contentLength", contentLength, "clientIP", clientIP, "clientHost", clientHost)
	for {
		n, readErr := reader.Read(buf)
		if readErr != nil {
			if readErr != io.EOF {
				slog.ErrorContext(reqHelper.ctx, "Error reading file", "error", readErr, "file", fileDisplayPath)
			}
			break
		}

		if n > 0 {
			_, writeErr := w.Write(buf[:n])
			if writeErr != nil {
				slog.ErrorContext(reqHelper.ctx, "Error writing response", "error", writeErr, "file", fileDisplayPath)
				break
			}
			totalSent += int64(n)

			reqHelper.publishDownloadProgress(int64(n))

			if time.Since(lastReportedTime) > downloadProgressLogInterval {
				totalMBSent = float64(totalSent) / 1024 / 1024

				mbps := float64(totalSent-lastReportedSent) / 1024 / 1024 / time.Since(lastReportedTime).Seconds()

				progress := float64(totalSent) / float64(info.Size()) * 100

				msg := fmt.Sprintf("%7.2f / %7.2f MB sent | %2.2f%% | %5.2f MB/s",
					totalMBSent, float64(contentLength)/1024/1024, progress, mbps)

				slog.InfoContext(reqHelper.ctx, msg, "file", fileDisplayPath)

				lastReportedSent = totalSent
				lastReportedTime = time.Now()
			}

		}
	}

	slog.InfoContext(reqHelper.ctx, "TRANSFER COMPLETE", "file", fileDisplayPath, "totalSent_mb", totalMBSent, "duration", time.Since(transferStart))
}

type reqHelper struct {
	w   http.ResponseWriter
	r   *http.Request
	ctx context.Context
	ch  chan<- ServerEvent
}

func newReqHelper(w http.ResponseWriter, r *http.Request, ch chan<- ServerEvent) *reqHelper {
	return &reqHelper{
		w:   w,
		r:   r,
		ctx: r.Context(),
		ch:  ch,
	}
}

func (h *reqHelper) attachReqId() *context.Context {
	reqId := nanoid.New()
	ctx := context.WithValue(h.ctx, utils.RequestIDKey, reqId)
	h.r = h.r.WithContext(ctx)
	h.ctx = ctx
	return &ctx
}

func (h *reqHelper) publishConnClose() {
	h.ch <- EventConnClose{
		ConnID: h.ctx.Value(utils.RequestIDKey).(string),
	}
}

func (h *reqHelper) publishDownloadProgress(sent int64) {
	h.ch <- EventDownloadProgress{
		ConnID: h.ctx.Value(utils.RequestIDKey).(string),
		Sent:   sent,
		Time:   time.Now(),
	}
}

func (h *reqHelper) publishDownloadStart(fileDisplayPath string, fileSize int64, rangeStart, rangeEnd int64, clientIP string, clientHost string) {
	h.ch <- EventDownloadStart{
		ConnID:          h.ctx.Value(utils.RequestIDKey).(string),
		FileDisplayPath: fileDisplayPath,
		Time:            time.Now(),
		TotalSize:       fileSize,
		Range:           Range{Start: rangeStart, End: rangeEnd},
		Client: &Client{
			IP:        clientIP,
			Host:      clientHost,
			UserAgent: h.r.UserAgent(),
		},
	}
}

var ErrInvalidRangeHeader = errors.New("invalid range header")

func (h *reqHelper) handleRange(file *os.File, fileInfo os.FileInfo) (startByte int64, contentLength int64, reader io.Reader, err error) {
	rng := h.r.Header.Get("Range")
	contentLength = fileInfo.Size()
	reader = file
	if rng != "" {
		hStartByte, endByte, parseErr := utils.ParseRangeHeader(rng, fileInfo.Size())
		if parseErr != nil {
			return 0, 0, nil, ErrInvalidRangeHeader
		}
		startByte = hStartByte

		if _, err := file.Seek(startByte, io.SeekStart); err != nil {
			return 0, 0, nil, err
		}

		contentLength = endByte - startByte + 1
		reader = io.LimitReader(file, contentLength)

		h.w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", startByte, endByte, fileInfo.Size()))
		h.w.WriteHeader(http.StatusPartialContent)

		slog.InfoContext(h.ctx, "PARTIAL", "path", h.r.URL.Path, "start", startByte, "end", endByte, "total", contentLength, logger.StatusCodeKey, http.StatusPartialContent)
	} else {
		slog.InfoContext(h.ctx, "OK", "path", h.r.URL.Path, "size", contentLength, logger.StatusCodeKey, http.StatusOK)
	}

	return startByte, contentLength, reader, nil
}

func (h *reqHelper) internalServerError(err error) {
	h.error("Internal Server Error", err, http.StatusInternalServerError)
}

func (h *reqHelper) error(mgs string, err error, statusCode int) {
	http.Error(h.w, mgs, statusCode)
	slog.ErrorContext(h.ctx, "", logger.StatusCodeKey, statusCode, "error", err)
}
