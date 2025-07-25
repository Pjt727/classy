package logginghelpers

import (
	"context"
	"errors"
	"log/slog"
)

// allows a single handler instance to call mutliple handlers
// prefering this over recursion as it feels much simpler especially when one handler may error
type MultiHandler struct {
	handlers []slog.Handler
}

func NewMultiHandler(handlers ...slog.Handler) *MultiHandler {
	return &MultiHandler{
		handlers: handlers,
	}
}

func (h *MultiHandler) AddHander(handler slog.Handler) {
	h.handlers = append(h.handlers, handler)
}

// if any is true return true
func (h *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// always calls each handlers returning all errors wrapped
func (h *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs error
	for _, handler := range h.handlers {
		err := handler.Handle(ctx, r)
		if err != nil && errs == nil {
			errs = err
		} else {
			errs = errors.Join(err)
		}
	}
	return errs
}

func (h *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithAttrs(attrs)
	}
	return &MultiHandler{handlers: newHandlers}
}

func (h *MultiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithGroup(name)
	}
	return &MultiHandler{handlers: newHandlers}
}
