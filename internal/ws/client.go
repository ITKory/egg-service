package ws

import (
	"chaos-egg/internal/logging"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
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
		c.hub.logger.Info("websocket client disconnected",
			zap.String("user_id", c.UserID),
			zap.String("username", c.Username),
		)
	}()

	c.conn.SetReadLimit(maxMessageSize)
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		c.hub.logger.Error("websocket read deadline failed",
			logging.Error(err),
			logging.ErrorLevelField(logging.ErrorLevelCritical),
		)
		return
	}
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure) {
				c.hub.logger.Warn("unexpected websocket close",
					logging.Error(err),
					logging.ErrorLevelField(logging.ErrorLevelRecoverable),
					zap.String("user_id", c.UserID),
				)
			}
			break
		}

		msg, err := Decode(message)
		if err != nil {
			c.hub.logger.Warn("invalid websocket message",
				logging.Error(err),
				logging.ErrorLevelField(logging.ErrorLevelExpected),
				zap.String("user_id", c.UserID),
			)
			continue
		}

		msg.UserID = c.UserID
		msg.Username = c.Username

		select {
		case c.hub.events <- msg:
		default:
			c.hub.logger.Warn("event channel full, dropping websocket message",
				logging.ErrorLevelField(logging.ErrorLevelRecoverable),
				zap.String("user_id", c.UserID),
				zap.String("message_type", string(msg.Type)),
			)
		}
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				c.hub.logger.Error("websocket write deadline failed",
					logging.Error(err),
					logging.ErrorLevelField(logging.ErrorLevelCritical),
					zap.String("user_id", c.UserID),
				)
				return
			}
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				c.hub.logger.Warn("websocket write failed",
					logging.Error(err),
					logging.ErrorLevelField(logging.ErrorLevelRecoverable),
					zap.String("user_id", c.UserID),
				)
				return
			}

		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				c.hub.logger.Error("websocket ping deadline failed",
					logging.Error(err),
					logging.ErrorLevelField(logging.ErrorLevelCritical),
					zap.String("user_id", c.UserID),
				)
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.hub.logger.Warn("websocket ping failed",
					logging.Error(err),
					logging.ErrorLevelField(logging.ErrorLevelRecoverable),
					zap.String("user_id", c.UserID),
				)
				return
			}
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
