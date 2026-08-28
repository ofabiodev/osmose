package gateway

import "github.com/gorilla/websocket"

func (c *Connection) readerLoop() {
	defer c.wg.Done()
	for {
		messageType, data, err := c.socket.ReadMessage()
		if err != nil {
			if c.ctx.Err() == nil {
				c.report(err)
			}
			return
		}
		if messageType != websocket.BinaryMessage {
			c.opts.Logger.Warn("ignoring non-binary Osmium frame", "type", messageType)
			continue
		}
		if c.opts.OnMessage != nil {
			c.opts.OnMessage(messageType, data)
		}
	}
}
