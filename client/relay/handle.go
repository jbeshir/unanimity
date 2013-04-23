package relay

import (
	"code.google.com/p/goprotobuf/proto"
)

import (
	"github.com/jbeshir/unanimity/client/relay/rproto"
	"github.com/jbeshir/unanimity/shared/connect"
)

var terminatedConnCh chan receivedConn

func init() {
	terminatedConnCh = make(chan receivedConn, 100)
}

func handleConn(node uint16, conn *connect.BaseConn) {

	for {
		msg, ok := <-conn.Received
		if !ok {
			break
		}

		// We only have one message type.
		if *msg.MsgType != 2 {
			conn.Close()
		}

		handleForward(conn, msg.Content)
	}

	terminatedConnCh <- receivedConn{node: node, conn: conn}
}

func handleForward(conn *connect.BaseConn, content []byte) {
	var msg rproto.Forward
	if err := proto.Unmarshal(content, &msg); err != nil {
		conn.Close()
		return
	}

	userMsg := new(UserMessage)
	userMsg.Sender = *msg.Sender
	userMsg.Recipient = *msg.Recipient
	userMsg.Tag = *msg.Tag
	userMsg.Content = *msg.Content
	userMsg.Ttl = uint16(*msg.Ttl)

	// Call message received callback.
	for _, cb := range receivedCallbacks {
		cb(userMsg)
	}
}
