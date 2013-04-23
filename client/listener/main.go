/*
	Package implementing the Client Protocol,
	listening for and creating incoming Client Protocol connections.

	Handles delivering user messages locally or passing them on
	to client/relay. Provides a startup function and a function to
	deliver a message.
*/
package listener

import (
	"github.com/jbeshir/unanimity/client/listener/cliproto_up"
	"github.com/jbeshir/unanimity/shared/connect"
	"github.com/jbeshir/unanimity/shared/store"
)

var connections map[*userConn]bool

func init() {
	connections = make(map[*userConn]bool)
}

type userConn struct {
	conn        *connect.BaseConn
	session     uint64
	waitingAuth *authData
	following   []uint64
}

type authData struct {
	msg       cliproto_up.Authenticate
	requestId uint64
	password  string
}

func Startup() {
	// Start accepting client protocol connections.
	go connect.Listen(connect.CLIENT_PROTOCOL, incomingConn)
}

func incomingConn(node uint16, conn *connect.BaseConn) {

	store.StartTransaction()
	if store.Degraded() {
		conn.Close()
		store.EndTransaction()
		return
	}

	userConn := new(userConn)
	userConn.conn = conn

	// Add to connections set.
	connections[userConn] = true

	store.EndTransaction()

	go handleConn(userConn)
}
