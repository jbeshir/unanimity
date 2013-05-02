/*
	Package implementing the Client Protocol,
	listening for and creating incoming Client Protocol connections.

	Handles delivering user messages locally or passing them on
	to client/relay. Provides a startup function and a function to
	deliver a message.
*/
package listener

import (
	"sync"
)

import (
	"github.com/jbeshir/unanimity/client/listener/cliproto_up"
	"github.com/jbeshir/unanimity/client/relay"
	"github.com/jbeshir/unanimity/shared/connect"
	"github.com/jbeshir/unanimity/shared/store"
)

var connections map[*userConn]bool

// Must be held when accessing the connections set.
// Must be locked after starting transaction if a transaction is to be entered,
// to impose an ordering on the two and avoid deadlocks.
var connectionsLock sync.Mutex

// Maps sessions to attached client connections.
var sessions map[uint64]*userConn

// Must be held when accessing the sessions set.
// Must be locked after starting transaction if a transaction is to be entered,
// to impose an ordering on the two and avoid deadlocks.
// Must be locked after connectionsLock if both are to be locked,
// to impose an ordering on the two and avoid deadlocks.
var sessionsLock sync.Mutex

func init() {
	connections = make(map[*userConn]bool)
}

type userConn struct {
	conn        *connect.BaseConn
	session     uint64
	waitingAuth *authData
	following   []uint64
	deliver     chan *relay.UserMessage
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
	userConn.deliver = make(chan *relay.UserMessage, 100)

	// Add to connections set.
	connectionsLock.Lock()
	connections[userConn] = true
	connectionsLock.Unlock()

	store.EndTransaction()

	go handleConn(userConn)
}
