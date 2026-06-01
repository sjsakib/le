package logger

import (
	"context"
	"log/slog"
	"os"

	"go.sakib.dev/le/pkg/utils"
)

const (
	StatusCodeKey string = "statusCode"
)

type Handler struct {
	slog.Handler
}

func NewHandler() *Handler {
	f, err := os.OpenFile(".le.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		panic("failed to open log file: " + err.Error())
	}
	f.Write([]byte("\n\n"))
	return &Handler{
		Handler: slog.NewTextHandler(f, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}),
	}
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	reqId, ok := ctx.Value(utils.RequestIDKey).(string)

	if !ok {
		return h.Handler.Handle(ctx, r)
	}

	r.AddAttrs(slog.String(string(utils.RequestIDKey), reqId))

	return h.Handler.Handle(ctx, r)
}
