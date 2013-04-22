package chrequest

import (
	"code.google.com/p/goprotobuf/proto"
)
import (
	"github.com/jbeshir/unanimity/config"
	"github.com/jbeshir/unanimity/shared/chrequest/chproto"
	"github.com/jbeshir/unanimity/shared/connect"
	"github.com/jbeshir/unanimity/shared/connect/baseproto"
)

type receivedForward struct {
	forward *chproto.ChangeForward
	node uint16
}

type receivedAck struct {
	ack *chproto.ChangeForwardAck
	node uint16
}

var terminatedConnCh chan receivedConn
var receivedForwardCh chan receivedForward
var receivedAckCh chan receivedAck

func init() {
	terminatedConnCh = make(chan receivedConn, 100)
	receivedForwardCh = make(chan receivedForward, 100)
	receivedAckCh = make(chan receivedAck, 100)
}

// Top-level function for the goroutine responsible for receiving from
// a change request protocol connection.
func handleConn(node uint16, conn *connect.BaseConn) {

	// Keep handling messages so long as the connection is active.
	for {
		msg, ok := <-conn.Received
		if !ok {
			break
		}

		handleMsg(node, conn, msg)
	}

	// Tell the processing goroutine that this connection has closed.
	// Causes it to stop sending to this connection.
	terminatedConnCh <- receivedConn{node: node, conn: conn}
}

func handleMsg(node uint16, conn *connect.BaseConn, msg *baseproto.Message) {
	if *msg.MsgType == 2 {
		// Change Forward message.
		forward := new(chproto.ChangeForward)
		err := proto.Unmarshal(msg.Content, forward)
		if err != nil {
			conn.Close()
		}

		// If this is not a core node,
		// and the requesting node is not this node,
		// discard the message.
		if uint16(*forward.Request.RequestNode) != config.Id() &&
			!config.IsCore() {
			return
		}

		// Send a Change Forward Ack message to the sender.
		ack := new(chproto.ChangeForwardAck)
		ack.RequestNode = forward.Request.RequestNode
		ack.RequestId = forward.Request.RequestId
		sendToConn(conn, 3, ack)

		// Send the forward message to our processing goroutine.
		receivedForwardCh <- receivedForward{forward: forward,
			node: node}

	} else if *msg.MsgType == 3 {
		// Change Forward Ack message.
		ack := new(chproto.ChangeForwardAck)
		err := proto.Unmarshal(msg.Content, ack)
		if err != nil {
			conn.Close()
		}

		// Send the ack message to our processing goroutine.
		receivedAckCh <- receivedAck{ack: ack, node: node}

	} else {
		// Unknown message.
		conn.Close()
	}
}
