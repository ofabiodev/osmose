package gateway

import (
	"time"

	"github.com/gorilla/websocket"
)

func (c *Connection) writerLoop() {
	defer c.wg.Done()
	for {
		select {
		case <-c.ctx.Done():
			return
		case frame := <-c.queue:
			if frame.Type == 0 {
				frame.Type = websocket.BinaryMessage
			}
			deadline := time.Now().Add(c.opts.WriteTimeout)
			if err := c.socket.SetWriteDeadline(deadline); err != nil {
				c.report(err)
				return
			}
			if err := c.socket.WriteMessage(frame.Type, frame.Data); err != nil {
				c.report(err)
				return
			}
		}
	}
}
