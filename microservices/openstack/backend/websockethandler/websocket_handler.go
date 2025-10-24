package websockethandler

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var Upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // à restreindre plus tard
	},
}

// Hub des connexions
type WebSocketHub struct {
	Clients map[string]*websocket.Conn
	Mu      sync.Mutex
}

// Instance globale du hub
var Hub = WebSocketHub{
	Clients: make(map[string]*websocket.Conn),
}

type WSMessage struct {
	Action string `json:"action"`
	Data   any    `json:"data"`
	Tag    string `json:"tag"`
}

// Envoi d’un message à un utilisateur
func SendMessageToUser(userID, action string, data any, tag string) {
	Hub.Mu.Lock()
	defer Hub.Mu.Unlock()

	if conn, ok := Hub.Clients[userID]; ok {
		message := WSMessage{Action: action, Data: data, Tag: tag}
		payload, err := json.Marshal(message)
		if err != nil {
			log.Println("Error marshalling WebSocket message:", err)
			return
		}
		conn.WriteMessage(websocket.TextMessage, payload)
	} else {
		log.Println("No active WebSocket connection for user", userID)
	}
}
