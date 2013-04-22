package chrequest

import (
	"github.com/jbeshir/unanimity/shared/chrequest/chproto"
	"github.com/jbeshir/unanimity/shared/connect"
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

	connections[node][0].SendProto(2, forward)
}
