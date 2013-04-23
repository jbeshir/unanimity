package consensus

import (
	"github.com/jbeshir/unanimity/shared/connect"
	"github.com/jbeshir/unanimity/shared/connect/baseproto"
)

type receivedMsg struct {
	node uint16
	conn *connect.BaseConn
	msg  *baseproto.Message
}

var receivedMsgCh chan receivedMsg
var terminatedConnCh chan receivedConn

func init() {
	receivedMsgCh = make(chan receivedMsg, 100)
	terminatedConnCh = make(chan receivedConn, 100)
}

// Keeps reading from the given consensus protocol connection until it closes.
func handleConn(node uint16, conn *connect.BaseConn) {
	for {
		msg, ok := <-conn.Received
		if !ok {
			break
		}

		receivedMsgCh <- receivedMsg{node: node, conn: conn, msg: msg}
	}

	terminatedConnCh <- receivedConn{node: node, conn: conn}
}
