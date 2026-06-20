package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"go.sakib.dev/le/logger"
	"go.sakib.dev/le/pkg/nanoid"
	"go.sakib.dev/le/pkg/utils"
)

type reqHelper struct {
	w          http.ResponseWriter
	r          *http.Request
	ctx        context.Context
	ch         chan<- ServerEvent
	absPath    string
	clientIP   string
	clientHost string
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
	h.ch <- &EventConnClose{
		ConnID: h.ctx.Value(utils.RequestIDKey).(string),
	}
}

func (h *reqHelper) publishDownloadProgress(sent int64) {
	h.ch <- &EventDownloadProgress{
		ConnID: h.ctx.Value(utils.RequestIDKey).(string),
		Sent:   sent,
		Time:   time.Now(),
	}
}

func (h *reqHelper) publishDownloadStart(fileDisplayPath string, fileSize int64, rangeStart, rangeEnd int64) {
	h.ch <- &EventDownloadStart{
		ConnID:          h.ctx.Value(utils.RequestIDKey).(string),
		FileDisplayPath: fileDisplayPath,
		Time:            time.Now(),
		TotalSize:       fileSize,
		Range:           Range{Start: rangeStart, End: rangeEnd},
		Client: &Client{
			IP:        h.clientIP,
			Host:      h.clientHost,
			UserAgent: h.r.UserAgent(),
		},
	}
}

var ErrInvalidRangeHeader = errors.New("invalid range header")

func (h *reqHelper) handleRange(source downloadSource) (startByte int64, contentLength int64, reader io.Reader, err error) {
	rng := h.r.Header.Get("Range")
	contentLength = -1
	reader = source

	resumableSource, isResumable := source.(resumableSource)

	if isResumable {
		contentLength = resumableSource.Size()
	}

	if rng == "" {
		slog.InfoContext(h.ctx, "OK", "path", h.r.URL.Path, "size", contentLength, logger.StatusCodeKey, http.StatusOK)
		return startByte, contentLength, reader, nil
	}

	if !isResumable {
		return 0, 0, nil, ErrInvalidRangeHeader
	}

	hStartByte, endByte, parseErr := utils.ParseRangeHeader(rng, contentLength)
	if parseErr != nil {
		return 0, 0, nil, ErrInvalidRangeHeader
	}
	startByte = hStartByte

	if _, err := resumableSource.SeekForward(startByte); err != nil {
		return 0, 0, nil, err
	}

	contentLength = endByte - startByte + 1
	reader = io.LimitReader(reader, contentLength)

	h.w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", startByte, endByte, resumableSource.Size()))

	slog.InfoContext(h.ctx, "PARTIAL", "path", h.r.URL.Path, "start", startByte, "end", endByte, "total", contentLength, logger.StatusCodeKey, http.StatusPartialContent)

	return startByte, contentLength, reader, nil
}

func (h *reqHelper) internalServerError(err error) {
	h.error("Internal Server Error", err, http.StatusInternalServerError)
}

func (h *reqHelper) error(mgs string, err error, statusCode int) {
	http.Error(h.w, mgs, statusCode)
	slog.ErrorContext(h.ctx, "", logger.StatusCodeKey, statusCode, "error", err)
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

	fileDisplayPath := utils.ReplaceHome(filepath.Join(filepath.Dir(rh.absPath), source.TargetName()))
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
