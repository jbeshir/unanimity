/*
	Package implementing the Relay Protocol, listening for and
	creating incoming and outgoing Relay Protocol connections.

	Provides a startup function and a function to relay a user message to
	the most appropriate other node, and a callback registration function
	for reacting to newly received user messages.
*/
package relay

import (
	"github.com/jbeshir/unanimity/client/relay/rproto"
	"github.com/jbeshir/unanimity/shared/connect"
	"github.com/jbeshir/unanimity/shared/store"
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
	// Unbuffered
	receivedConnCh = make(chan receivedConn)
}

type UserMessage struct {
	Sender, Recipient uint64
	Tag               string
	Content           string
	Ttl               uint16
}

func Startup() {
	// Start processing goroutine.
	go process()

	// Start accepting change request connections.
	go connect.Listen(connect.RELAY_PROTOCOL, incomingConn)
}

func Forward(node uint16, userMsg *UserMessage) {
	store.StartTransaction()
	defer store.EndTransaction()

	// While degraded, we drop all messages.
	if store.Degraded() {
		return
	}

	// If we have no connection to that node, we drop the message.
	if len(connections[node]) == 0 {
		return
	}

	// Otherwise, send to the given node.
	var forward rproto.Forward
	forward.Sender = new(uint64)
	forward.Recipient = new(uint64)
	forward.Tag = new(string)
	forward.Content = new(string)
	forward.Ttl = new(uint32)
	*forward.Sender = userMsg.Sender
	*forward.Recipient = userMsg.Recipient
	*forward.Tag = userMsg.Tag
	*forward.Content = userMsg.Content
	*forward.Ttl = uint32(userMsg.Ttl)

	connections[node][0].SendProto(2, &forward)
}

func incomingConn(node uint16, conn *connect.BaseConn) {

	receivedConnCh <- receivedConn{node: node, conn: conn}
	go handleConn(node, conn)
}
