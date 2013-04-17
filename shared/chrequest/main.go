/*
	Package implementing the Change Request Protocol,
	listening for and creating incoming and outgoing
	Change Request Protocol connections,
	and managing receiving and forwarding change requests.

	Uses shared/store’s callback registration functions to
	react to newly added instructions.

	Provides a startup function, methods to create new change requests
	and a change channel to receive change requests that this node is
	the leader for, applicable only to core nodes.
*/
package chrequest

import (
	"github.com/jbeshir/unanimity/shared/connect"
)

func Startup() {

	// Start accepting change request connections.
	go connect.Listen(connect.CHANGE_REQUEST_PROTOCOL, newChangeRequestConn)
}

func newChangeRequestConn(node uint16, baseConn *connect.BaseConn) {
	baseConn.Close()
}
