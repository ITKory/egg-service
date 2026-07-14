package ws

import (
	"log"

	"github.com/gorilla/websocket"
)

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	UserID   string
	Username string
}

func NewClient(hub *Hub, conn *websocket.Conn, userID, username string) *Client {
	return &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, 256),
		UserID:   userID,
		Username: username,
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		msg, err := Decode(message)
		if err != nil {
			log.Printf("Invalid message: %v", err)
			continue
		}

		msg.UserID = c.UserID
		msg.Username = c.Username

		select {
		case c.hub.events <- msg:
		default:
			log.Printf("Event channel full, dropping message from %s", c.UserID)
		}
	}
}

func (c *Client) WritePump() {
	defer func() {
		c.conn.Close()
	}()

	for message := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Printf("WebSocket write error: %v", err)
			return
		}
	}
}

func (c *Client) Send(msg Message) {
	data, err := Encode(msg)
	if err != nil {
		return
	}
	c.send <- data
}
