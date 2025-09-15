package servermanage

import (
	"bytes"
	"context"
	"net/http"

	"log/slog"
	"slices"

	"github.com/Pjt727/classy/data/db"
	"github.com/Pjt727/classy/server/components"
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

type websocketLoggingWriter struct {
	orchestratorLabel int
	termCollection    db.TermCollection
	serviceName       string
	h                 *manageHandler
	ctx               context.Context
}

func newWebSocketWriter(
	ctx context.Context,
	orchestratorLabel int,
	termCollection db.TermCollection,
	serviceName string,
	h *manageHandler,
) *websocketLoggingWriter {
	return &websocketLoggingWriter{
		orchestratorLabel: orchestratorLabel,
		termCollection:    termCollection,
		serviceName:       serviceName,
		h:                 h,
		ctx:               ctx,
	}
}

func (w *websocketLoggingWriter) Write(b []byte) (int, error) {
	// it is completely fine if the log siliently does not get sent
	wsConn, ok := w.h.orchestrators[w.orchestratorLabel]
	if !ok {
		slog.Warn("ws failed to be established")
		return 0, nil
	}

	bytesLen := len(b)
	formattedLog := ansihtml.ConvertToHTML(b)

	var logNotification bytes.Buffer
	err := components.CollectionLog(w.termCollection, string(formattedLog)).Render(w.ctx, &logNotification)
	if err != nil {
		slog.Error("could render log", "err", err)
		return bytesLen, err
	}
	for _, c := range wsConn.connections {
		if c == nil || c.send == nil {
			continue
		}
		c.send <- logNotification.Bytes()
	}

	return bytesLen, nil
}

func (w *websocketLoggingWriter) start(ctx context.Context) error {
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

func (w *websocketLoggingWriter) finish(ctx context.Context, status components.JobStatus) error {
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
		slog.Error("Could not render the finsihed oob for log", "err", err)
		return err
	}

	for _, c := range wsConn.connections {
		if c == nil || c.send == nil {
			continue
		}

		c.send <- buf.Bytes()
	}
	return nil
}

type WebSocketConnection struct {
	conn              *websocket.Conn
	orchestratorLabel int
	send              chan []byte
	h                 *manageHandler
}

func (h *manageHandler) loggingWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	requestContext := r.Context()
	if err != nil {
		slog.Info("Could not upgrade", "err", err)
		return
	}

	label := requestContext.Value(OrchestratorLabel).(int)

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
	for message := range wsConn.send {
		err := wsConn.conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			slog.Error("Channel error: ", "err", err)
			return
		}
	}
	wsConn.conn.WriteMessage(websocket.CloseMessage, []byte{})
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
