package controllers

import (
	"PoolManagerVM/backend/websockethandler"
	"fmt"

	"github.com/gin-gonic/gin"
)

func HandleWebSocket(c *gin.Context) {
	userID := c.GetString("email")

	conn, err := websockethandler.Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println("Erreur upgrade:", err)
		return
	}

	websockethandler.Hub.Mu.Lock()
	websockethandler.Hub.Clients[userID] = conn
	websockethandler.Hub.Mu.Unlock()
	fmt.Printf("🟢 Connexion WebSocket ouverte pour %s\n", userID)

	defer func() {
		conn.Close()
		websockethandler.Hub.Mu.Lock()
		delete(websockethandler.Hub.Clients, userID)
		websockethandler.Hub.Mu.Unlock()
		fmt.Printf("🔴 Connexion fermée pour %s\n", userID)
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("Erreur lecture:", err)
			break
		}
		fmt.Printf("📨 Message de %s: %s\n", userID, msg)
	}
}
