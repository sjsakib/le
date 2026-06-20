package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"go.sakib.dev/le/pkg/utils"
)

const (
	StatusCodeKey string = "statusCode"
)

type Handler struct {
	slog.Handler

	isEnabled bool
	file      *os.File
}

func NewHandler(path string) *Handler {
	var w io.Writer = io.Discard
	if path != "" {
		path = utils.ExpandHome(path)
		if err := os.MkdirAll(path, 0755); err != nil {
			panic("failed to create log directory: " + err.Error())
		}
		f, err := os.OpenFile(filepath.Join(path, "le.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			panic("failed to open log file: " + err.Error())
		}
		w = f
	}
	w.Write([]byte("\n\n"))
	return &Handler{
		Handler: slog.NewTextHandler(w, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}),
		isEnabled: path != "",
	}
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	if !h.isEnabled {
		return nil
	}
	reqId, ok := ctx.Value(utils.RequestIDKey).(string)

	if !ok {
		return h.Handler.Handle(ctx, r)
	}

	r.AddAttrs(slog.String(string(utils.RequestIDKey), reqId))

	return h.Handler.Handle(ctx, r)
}

func (h *Handler) Close() error {
	if !h.isEnabled {
		return nil
	}
	return h.file.Close()
}
