package chrequest

import (
	"code.google.com/p/goprotobuf/proto"
)

import (
	"github.com/jbeshir/unanimity/shared/chrequest/chproto"
	"github.com/jbeshir/unanimity/shared/connect"
	"github.com/jbeshir/unanimity/shared/connect/baseproto"
)

// May only be accessed by the processing goroutine.
var connections = make(map[uint16][]*connect.BaseConn)

// Sends the given change forward message to the given node.
// Must not be called with our own node ID.
// Does not handle adding timeouts, or adding this node ID to the ignore list.
// Must be called from the processing goroutine.
func sendForward(node uint16, forward *chproto.ChangeForward) {

	// If we don't have a connection to this node,
	// establish an outgoing connection.
	if len(connections[node]) == 0 {
		conn, err := connect.Dial(connect.CHANGE_REQUEST_PROTOCOL, node)
		if err != nil {
			// Message is dropped.
			return
		}

		connections[node] = append(connections[node], conn)
		go handleConn(node, conn)
	}
	
	sendToConn(connections[node][0], 2, forward)
}

// Sends the given change protocol message to the given node.
func sendToConn(conn *connect.BaseConn, msgType uint32, msg proto.Message) {

	content, err := proto.Marshal(msg)
	if err != nil {
		conn.Close()
		return
	}

	baseMsg := new(baseproto.Message)
	baseMsg.MsgType = new(uint32)
	*baseMsg.MsgType = msgType
	baseMsg.Content = content

	conn.Send(baseMsg)
}
