/*
	Package implementing the Follow Protocol,
	listening for and creating incoming and outgoing
	Follow Protocol connections.

	Uses shared/store’s callback registration functions to
	react to a variety of events.

	Provides a startup function.
*/
package follow

import (
	"github.com/jbeshir/unanimity/shared/connect"
)

// Must be unbuffered.
// This ensures that we block until the processing goroutine has received
// the message, which ensures any terminate message arrives after the
// received message.
var receivedConnCh chan *followConn

func init() {
	// Unbuffered.
	receivedConnCh = make(chan *followConn)
}

func Startup() {

	// Start processing goroutine.
	go process()

	// Start accepting change request connections.
	go connect.Listen(connect.FOLLOW_PROTOCOL, incomingConn)
}

func incomingConn(node uint16, conn *connect.BaseConn) {

	followConn := new(followConn)
	followConn.node = node
	followConn.conn = conn

	receivedConnCh <- followConn
}
