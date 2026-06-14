package server

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"go.sakib.dev/le/pkg/utils"
	"go.sakib.dev/le/pkg/zip"
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
	reqHelper.clientIP = clientIP

	clientHost, err := utils.GetClientHostname(r)
	if err != nil {
		slog.Warn("Failed to get client hostname", "error", err)
		clientHost = "unknown"
	}
	reqHelper.clientHost = clientHost

	slog.InfoContext(reqHelper.ctx, "REQUEST",
		"clientIP", clientIP,
		"clientHost", clientHost,
		"userAgent", r.UserAgent(),
		"method", r.Method,
		"path", r.URL.Path)

	defer reqHelper.publishConnClose()

	isHead := r.Method == http.MethodHead

	if r.Method != http.MethodGet && !isHead {
		reqHelper.error("Method Not Allowed", nil, http.StatusMethodNotAllowed)
		return
	}

	absPath, err := utils.SecureJoin(string(h.root), r.URL.Path)
	reqHelper.absPath = absPath

	slog.Debug("Secure Join", "path", absPath, "root", string(h.root), "path", r.URL.Path, "error", err)

	if errors.Is(err, utils.ErrForbiddenPath) {
		reqHelper.error("NOT FOUND", err, http.StatusNotFound)
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

	isArchive := r.URL.Query().Get("archive") == "true"

	if info.IsDir() && !isArchive {
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

	var source downloadSource

	if isArchive && info.IsDir() {
		source = zip.New(absPath, r.URL.Query().Get("compressed") == "true")
	} else {
		file, err := os.Open(absPath)
		if err != nil {
			reqHelper.internalServerError(err)
			return
		}
		defer file.Close()

		source = &fileSource{file, info}

	}

	reqHelper.serveSource(source)
}

func (rh *reqHelper) serveSource(source downloadSource) {
	var transferStart = time.Now()
	startByte, contentLength, reader, err := rh.handleRange(source)
	if err != nil {
		if errors.Is(err, ErrInvalidRangeHeader) {
			rh.error("Invalid Range", err, http.StatusRequestedRangeNotSatisfiable)
			return
		}
		rh.internalServerError(err)
		return

	}

	rh.w.Header().Set("Content-Type", "application/octet-stream")
	rh.w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", source.TargetName()))

	resumable, isResumable := source.(resumableSource)

	var fileSize int64 = -1
	if isResumable {
		fileSize = resumable.Size()
		rh.w.Header().Set("Accept-Ranges", "bytes")
		rh.w.Header().Set("Content-Length", fmt.Sprintf("%d", contentLength))
		rh.w.Header().Set("ETag", resumable.ETag())

		if contentLength != fileSize {
			rh.w.WriteHeader(http.StatusPartialContent)
		}
	}

	fileDisplayPath := utils.ReplaceHome(rh.absPath)
	var totalSent int64 = 0
	var totalMBSent float64
	buf := make([]byte, 1024*1024) // 1MB buffer
	var lastReportedSent int64 = 0
	var lastReportedTime = time.Now()

	if rh.r.Method == http.MethodHead {
		rh.w.WriteHeader(http.StatusOK)
		return
	}

	rh.publishDownloadStart(fileDisplayPath, fileSize, startByte, startByte+contentLength-1)
	slog.Info("Starting download", "file", fileDisplayPath, "totalSize", fileSize, "rangeStart", startByte, "contentLength", contentLength, "clientIP", rh.clientIP, "clientHost", rh.clientHost)
	for {
		n, err := reader.Read(buf)
		if err != nil {
			if err != io.EOF {
				slog.ErrorContext(rh.ctx, "Error reading file", "error", err, "file", fileDisplayPath)
			}
			break
		}

		if n > 0 {
			_, err := rh.w.Write(buf[:n])
			if err != nil {
				slog.ErrorContext(rh.ctx, "Error writing response", "error", err, "file", fileDisplayPath)
				break
			}
			totalSent += int64(n)

			rh.publishDownloadProgress(int64(n))

			if time.Since(lastReportedTime) > downloadProgressLogInterval {
				totalMBSent = float64(totalSent) / 1024 / 1024

				mbps := float64(totalSent-lastReportedSent) / 1024 / 1024 / time.Since(lastReportedTime).Seconds()

				progress := float64(totalSent) / float64(fileSize) * 100

				msg := fmt.Sprintf("%7.2f / %7.2f MB sent | %2.2f%% | %5.2f MB/s",
					totalMBSent, float64(contentLength)/1024/1024, progress, mbps)

				slog.InfoContext(rh.ctx, msg, "file", fileDisplayPath)

				lastReportedSent = totalSent
				lastReportedTime = time.Now()
			}

		}
	}

	slog.InfoContext(rh.ctx, "TRANSFER COMPLETE", "file", fileDisplayPath, "totalSent_mb", totalMBSent, "duration", time.Since(transferStart))
}
