package handlers

import (
	"bytes"
	"net/http"

	"github.com/Pjt727/classy/api/components"
	"github.com/Pjt727/classy/data/db"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgtype"
	log "github.com/sirupsen/logrus"
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
	termID            string
	schoolID          string
}

func (w *WebsocketLoggingHook) Levels() []log.Level {
	return log.AllLevels
}

func (w *WebsocketLoggingHook) Fire(e *log.Entry) error {
	wsConn, ok := orchestrators[w.orchestratorLabel]

	// it is completely fine if the log does not get sent
	if !ok {
		return nil
	}
	formattedLog, err := e.String()
	if err != nil {
		return err
	}

	for _, c := range wsConn.connections {
		c.send <- []byte(formattedLog)
	}
	return nil
}

func (w *WebsocketLoggingHook) start(e *log.Entry) error {
	wsConn, ok := orchestrators[w.orchestratorLabel]

	// it is completely fine if the log does not get sent
	if !ok {
		return nil
	}

	var buf bytes.Buffer
	err := components.ActiveTermCollection(db.TermCollection{
		ID:              w.termID,
		SchoolID:        w.schoolID,
		Year:            0,
		Season:          "",
		Name:            pgtype.Text{},
		StillCollecting: false,
	}, true).Render(e.Context, &buf)
	if err != nil {
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
}

func (h *ManageHandler) LoggingWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	ctx := r.Context()
	if err != nil {
		log.Println(err)
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
	}

	// creation
	{
		log.Println("Added webconnection")
		orch := orchestrators[label]
		orch.mu.Lock()
		defer orch.mu.Unlock()
		orch.connections = append(orch.connections, wsConn)
	}

	// running of the websocket
	go wsConn.writePump()
	go wsConn.readPump()
}

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
	defer func() {
		wsConn.disconnect()
	}()
	for {
		select {
		case message, ok := <-wsConn.send:
			if !ok {
				// The hub closed the channel.
				wsConn.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			log.Println(string(message))

			err := wsConn.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Error(err)
				return
			}
		}
	}
}

func (wsConn *WebSocketConnection) disconnect() {
	log.Println("broke down webconnection")
	orch := orchestrators[wsConn.orchestratorLabel]
	orch.mu.Lock()
	defer orch.mu.Unlock()
	for i, c := range orch.connections {
		if c == wsConn {
			orch.connections = append(orch.connections[:i], orch.connections[i+1:]...)
			break
		}
	}
}
