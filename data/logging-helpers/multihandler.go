package logginghelpers

import (
	"context"
	"errors"
	"log/slog"
)

// allows a single handler instance to call mutliple handlers
// prefering this over recursion as it feels much simpler especially when one handler may error
// adding a handler will keep all of the groups added from this logger
type MultiHandler struct {
	handlers          []slog.Handler
	accumulatedAttrs  []slog.Attr
	accumulatedGroups []string
}

func NewMultiHandler(handlers ...slog.Handler) *MultiHandler {
	return &MultiHandler{
		handlers: handlers,
	}
}

// replays all groups and attributes add to the multihandler to the new handler
// and returns the new handler
func (h *MultiHandler) WithHandler(newHandler slog.Handler) *MultiHandler {
	for _, groupName := range h.accumulatedGroups {
		newHandler = newHandler.WithGroup(groupName)
	}
	newHandler = newHandler.WithAttrs(h.accumulatedAttrs)

	updatedHandlers := make([]slog.Handler, len(h.handlers)+1)
	copy(updatedHandlers, h.handlers)
	updatedHandlers[len(h.handlers)] = newHandler

	return &MultiHandler{
		handlers:          updatedHandlers,
		accumulatedAttrs:  h.accumulatedAttrs,
		accumulatedGroups: h.accumulatedGroups,
	}
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
		errs = errors.Join(errs, handler.Handle(ctx, r))
	}
	return errs
}

func (h *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithAttrs(attrs)
	}

	newMultiHandler := &MultiHandler{
		handlers:          newHandlers,
		accumulatedAttrs:  make([]slog.Attr, len(h.accumulatedAttrs)+len(attrs)),
		accumulatedGroups: make([]string, len(h.accumulatedGroups)),
	}
	copy(newMultiHandler.accumulatedAttrs, h.accumulatedAttrs)
	copy(newMultiHandler.accumulatedAttrs[len(h.accumulatedAttrs):], attrs)
	copy(newMultiHandler.accumulatedGroups, h.accumulatedGroups)

	return newMultiHandler
}

func (h *MultiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithGroup(name)
	}

	newMultiHandler := &MultiHandler{
		handlers:          newHandlers,
		accumulatedAttrs:  make([]slog.Attr, len(h.accumulatedAttrs)),
		accumulatedGroups: make([]string, len(h.accumulatedGroups)+1),
	}
	copy(newMultiHandler.accumulatedAttrs, h.accumulatedAttrs)
	copy(newMultiHandler.accumulatedGroups, h.accumulatedGroups)
	newMultiHandler.accumulatedGroups[len(h.accumulatedGroups)] = name

	return newMultiHandler
}

// logger's handler should be multihandler for groups/ attrs to be replayed
func WithHandler(logger *slog.Logger, handler slog.Handler) *slog.Logger {
	loggerHandler := logger.Handler()
	switch h := loggerHandler.(type) {
	case *MultiHandler:
		return slog.New(h.WithHandler(handler))
	default:
		// maybe this should be an acceptable use case
		slog.Warn("logger does not have the right handler type not replaying groups/ attrs")
		return slog.New(NewMultiHandler(loggerHandler, handler))
	}
}
