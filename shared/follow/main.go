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
	"log"
)

import (
	"github.com/jbeshir/unanimity/shared/connect"
)

var receivedConnCh chan *followConn

func init() {
	receivedConnCh = make(chan *followConn, 100)
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

	log.Print("shared/follow: received connection from ", node)
	receivedConnCh <- followConn
}
