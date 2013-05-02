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

// Must be held when accessing the sessions map.
// Must be locked after starting transaction if a transaction is to be entered,
// to impose an ordering on the two and avoid deadlocks.
// Must be locked after connectionsLock if both are to be locked,
// to impose an ordering on the two and avoid deadlocks.
var sessionsLock sync.Mutex

// Maps request IDs to connections waiting for them to complete.
var waiting map[uint64]*userConn

// Must be held when accessing the waiting map.
// Must be locked after starting transaction if a transaction is to be entered,
// to impose an ordering on the two and avoid deadlocks.
// Must be locked after connectionsLock if both are to be locked,
// to impose an ordering on the two and avoid deadlocks.
var waitingLock sync.Mutex

func init() {
	connections = make(map[*userConn]bool)
	sessions = make(map[uint64]*userConn)
	waiting = make(map[uint64]*userConn)
}

type userConn struct {
	conn    *connect.BaseConn
	deliver chan *relay.UserMessage

	// Must only be accessed during a transaction.
	following []uint64

	// Must only be accessed holding the sessions lock.
	session uint64

	// Must only be accessed holding the waiting lock.
	waitingAuth *authData
}

type authData struct {
	msg       cliproto_up.Authenticate
	requestId uint64
	password  string
}

func Startup() {
	// If undegraded...
	// - TODO: Check for attached sessions we lack connections for.
	// - TODO: Check for nameless users.

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
