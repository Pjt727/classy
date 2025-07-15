package handlers

import (
	"bytes"
	"context"
	"net/http"

	"github.com/Pjt727/classy/api/components"
	"github.com/Pjt727/classy/data/db"
	"github.com/gorilla/websocket"
	"github.com/robert-nix/ansihtml"
	log "github.com/sirupsen/logrus"
	"slices"
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

type WebsocketLoggingHook struct {
	orchestratorLabel int
	termCollection    db.TermCollection
	serviceName       string
	h                 *ManageHandler
}

func (w *WebsocketLoggingHook) Levels() []log.Level {
	return log.AllLevels
}

func (w *WebsocketLoggingHook) Fire(e *log.Entry) error {
	wsConn, ok := w.h.orchestrators[w.orchestratorLabel]
	// it is completely fine if the log does not get sent
	if !ok {
		log.Warn("ws failed to be established")
		return nil
	}

	logString, err := e.String()
	if err != nil {
		log.Error(err)
		return err
	}
	formattedLog := ansihtml.ConvertToHTML([]byte(logString))
	var buf bytes.Buffer
	err = components.CollectionLog(w.termCollection, string(formattedLog)).Render(e.Context, &buf)
	if err != nil {
		log.Error(err)
		return err
	}
	for _, c := range wsConn.connections {
		if c == nil {
			continue
		}
		c.send <- buf.Bytes()
	}
	return nil
}

func (w *WebsocketLoggingHook) start(ctx context.Context) error {
	log.Info("Starting term collection")
	wsConn, ok := w.h.orchestrators[w.orchestratorLabel]

	// it is completely fine if the log does not get sent
	if !ok {
		log.Warn("Could not find the orch")
		return nil
	}

	var buf bytes.Buffer
	err := components.ActiveTermCollectionOob(w.termCollection).Render(ctx, &buf)
	if err != nil {
		log.Error("Could not render the starting oob", err)
		return err
	}

	for _, c := range wsConn.connections {
		if c == nil {
			continue
		}
		c.send <- buf.Bytes()
	}
	log.Info("Finished term collection")
	return nil
}

func (w *WebsocketLoggingHook) finish(ctx context.Context, status components.JobStatus) error {
	wsConn, ok := w.h.orchestrators[w.orchestratorLabel]

	// it is completely fine if the log does not get sent
	if !ok {
		log.Warn("Could not find the orch")
		return nil
	}

	var buf bytes.Buffer
	err := components.JobFinished(w.orchestratorLabel, w.serviceName, w.termCollection, status).
		Render(ctx, &buf)
	if err != nil {
		log.Error("Could not render the finsihed oob", err)
		return err
	}

	for _, c := range wsConn.connections {
		c.send <- buf.Bytes()
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
		log.Println("Could not upgrade: ", err)
		return
	}

	label := ctx.Value(OrchestratorLabel).(int)
	userCookie := ctx.Value(UserCookie).(string)

	if userCookie == "" {
		log.Error("User cookie not found")
		conn.Close()
		return
	}

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
				log.Printf("error: %v", err)
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
				log.Error("Channel error: ", err)
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
