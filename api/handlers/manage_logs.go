package handlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"log/slog"
	"slices"

	"github.com/Pjt727/classy/api/components"
	"github.com/Pjt727/classy/data/db"
	"github.com/gorilla/websocket"
	"github.com/robert-nix/ansihtml"
)

// logging is designed such that even if the user destroys their websocket
//    and then comes back they will see the job they are running as get a stream
//    of the logs
// it also should work if multiple users are interacting with the same orchestrator
// each orchestrator has a list of websockets to send out each command and each
// logs are OK to be lost while the process is still running because they could be accessed after the fact

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins
	},
}

type websocketLoggingHandler struct {
	orchestratorLabel int
	termCollection    db.TermCollection
	serviceName       string
	h                 *ManageHandler
	innerHandler      slog.Handler
}

func NewWebSocketHandler(
	orchestratorLabel int,
	termCollection db.TermCollection,
	serviceName string,
	h *ManageHandler,
	innerHandler slog.Handler,
) *websocketLoggingHandler {
	return &websocketLoggingHandler{
		orchestratorLabel: orchestratorLabel,
		termCollection:    termCollection,
		serviceName:       serviceName,
		h:                 h,
		innerHandler:      innerHandler,
	}
}

func (w *websocketLoggingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	// Custom logic before calling the wrapped handler's Enabled method
	return w.innerHandler.Enabled(ctx, level)
}

func (w *websocketLoggingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &websocketLoggingHandler{
		orchestratorLabel: w.orchestratorLabel,
		termCollection:    w.termCollection,
		serviceName:       w.serviceName,
		h:                 w.h,
		innerHandler:      w.innerHandler.WithAttrs(attrs),
	}
}

func (w *websocketLoggingHandler) WithGroup(name string) slog.Handler {
	return &websocketLoggingHandler{
		orchestratorLabel: w.orchestratorLabel,
		termCollection:    w.termCollection,
		serviceName:       w.serviceName,
		h:                 w.h,
		innerHandler:      w.innerHandler.WithGroup(name),
	}
}

type formatter struct {
	buf []byte
}

func (f *formatter) appendString(s string) {
	f.buf = append(f.buf, s...)
}

func (f *formatter) appendValue(v slog.Value) {
	switch v.Kind() {
	case slog.KindString:
		f.buf = append(f.buf, v.String()...)
	case slog.KindInt64:
		f.buf = strconv.AppendInt(f.buf, v.Int64(), 10)
	case slog.KindUint64:
		f.buf = strconv.AppendUint(f.buf, v.Uint64(), 10)
	case slog.KindFloat64:
		f.buf = strconv.AppendFloat(f.buf, v.Float64(), 'g', -1, 64)
	case slog.KindBool:
		f.buf = strconv.AppendBool(f.buf, v.Bool())
	case slog.KindDuration:
		f.appendString(v.Duration().String())
	case slog.KindTime:
		t := v.Time()
		f.buf = t.AppendFormat(f.buf, time.RFC3339Nano)
	case slog.KindAny:
		a := v.Any()
		if err, ok := a.(error); ok {
			f.appendString(err.Error())
		} else if s, ok := a.(fmt.Stringer); ok {
			f.appendString(s.String())
		} else {
			f.appendString(fmt.Sprint(a))
		}
	default:
		panic(fmt.Sprintf("bad kind: %s", v.Kind()))
	}
}

func lightCyan(s string) string {
	return "\033[96m" + s + "\033[0m"
}

func (w *websocketLoggingHandler) Handle(ctx context.Context, r slog.Record) error {
	wsConn, ok := w.h.orchestrators[w.orchestratorLabel]
	// it is completely fine if the log does not get sent
	if !ok {
		slog.Warn("ws failed to be established")
		return nil
	}

	runningLog := &formatter{
		buf: []byte{},
	}
	runningLog.appendString(r.Message)
	runningLog.appendString(" ")
	r.Attrs(func(a slog.Attr) bool {
		runningLog.appendString(lightCyan(a.Key))
		runningLog.appendString("=")
		runningLog.appendValue(a.Value)
		runningLog.appendString(" ")
		return true
	})

	formattedLog := ansihtml.ConvertToHTML(runningLog.buf)

	var logNotification bytes.Buffer
	err := components.CollectionLog(w.termCollection, string(formattedLog)).Render(ctx, &logNotification)
	if err != nil {
		slog.Error("could render log", "err", err)
		return err
	}
	for _, c := range wsConn.connections {
		if c == nil || c.send == nil {
			continue
		}
		select {
		case c.send <- logNotification.Bytes():
		default:
		}
	}
	return w.innerHandler.Handle(ctx, r)
}

func (w *websocketLoggingHandler) start(ctx context.Context) error {
	slog.Info("Starting term collection")
	wsConn, ok := w.h.orchestrators[w.orchestratorLabel]

	// it is completely fine if the slog.does not get sent
	if !ok {
		slog.Warn("Could not find the orch")
		return nil
	}

	var buf bytes.Buffer
	err := components.ActiveTermCollectionOob(w.termCollection).Render(ctx, &buf)
	if err != nil {
		slog.Error("Could not render the starting oob", "err", err)
		return err
	}

	for _, c := range wsConn.connections {
		if c == nil || c.send == nil {
			continue
		}
		select {
		case c.send <- buf.Bytes():
		default:
		}
	}
	slog.Info("Finished term collection")
	return nil
}

func (w *websocketLoggingHandler) finish(ctx context.Context, status components.JobStatus) error {
	wsConn, ok := w.h.orchestrators[w.orchestratorLabel]

	// it is completely fine if the slog.does not get sent
	if !ok {
		slog.Warn("Could not find the orch")
		return nil
	}

	var buf bytes.Buffer
	err := components.JobFinished(w.orchestratorLabel, w.serviceName, w.termCollection, status).
		Render(ctx, &buf)
	if err != nil {
		slog.Error("Could not render the finsihed oob", "err", err)
		return err
	}

	for _, c := range wsConn.connections {
		if c == nil || c.send == nil {
			continue
		}
		select {
		case c.send <- buf.Bytes():
		default:
		}
	}
	return nil
}

type WebSocketConnection struct {
	conn              *websocket.Conn
	orchestratorLabel int
	send              chan []byte
	h                 *ManageHandler
}

func (h *ManageHandler) LoggingWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	ctx := r.Context()
	if err != nil {
		slog.Info("Could not upgrade", "err", err)
		return
	}

	label := ctx.Value(OrchestratorLabel).(int)

	wsConn := &WebSocketConnection{
		conn:              conn,
		orchestratorLabel: label,
		send:              make(chan []byte),
		h:                 h,
	}

	// creation
	{
		orch := h.orchestrators[label]
		orch.mu.Lock()
		defer orch.mu.Unlock()
		orch.connections = append(orch.connections, wsConn)
	}

	// running of the websocket
	go wsConn.writePump()
	// go wsConn.readPump()
}

// TODO: Implement cancellactions of collections
func (wsConn *WebSocketConnection) readPump() {
	defer func() {
		wsConn.disconnect()
	}()
	for {
		_, _, err := wsConn.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(
				err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				slog.Info("error: %v", "err", err)
			}
			break
		}
		// might add cancellation options
	}
}

func (wsConn *WebSocketConnection) writePump() {
	defer wsConn.disconnect()
	for {
		select {
		case message, ok := <-wsConn.send:
			if !ok {
				// The hub closed the channel.
				wsConn.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := wsConn.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				slog.Error("Channel error: ", "err", err)
				return
			}
		}
	}
}

func (wsConn *WebSocketConnection) disconnect() {
	orch := wsConn.h.orchestrators[wsConn.orchestratorLabel]
	orch.mu.Lock()
	defer orch.mu.Unlock()
	wsConn.conn.Close()
	close(wsConn.send)
	for i, c := range orch.connections {
		if c == wsConn {
			orch.connections = slices.Delete(orch.connections, i, i+1)
			break
		}
	}
}
