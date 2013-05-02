/*
	Package implementing the Consensus Protocol,
	listening for and creating incoming and outgoing Consensus Protocol
	connections.

	Provides a startup function, and attaches to the change request
	callback provided by shared/chrequest. Filters out changes whose
	change request ID already occurs in our instruction proposals.
*/
package consensus

import (
	"log"
)

import (
	"github.com/jbeshir/unanimity/shared/connect"
)

type receivedConn struct {
	node uint16
	conn *connect.BaseConn
}

// Must be unbuffered.
// This ensures that we block until the processing goroutine has received
// the message, which ensures any terminate message arrives after the
// received message.
var receivedConnCh chan receivedConn

func init() {
	// Unbuffered.
	receivedConnCh = make(chan receivedConn)
}

func Startup() {

	// Start processing goroutine.
	go process()

	// Start accepting change request connections.
	go connect.Listen(connect.CONSENSUS_PROTOCOL, incomingConn)
}

func incomingConn(node uint16, conn *connect.BaseConn) {

	receivedConnCh <- receivedConn{node: node, conn: conn}

	log.Print("core/consensus: received connection from ", node)
	handleConn(node, conn)
}
