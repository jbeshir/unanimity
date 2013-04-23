package listener

import (
	"github.com/jbeshir/unanimity/shared/store"
)

func handleConn(conn *userConn) {
	for {
		_, ok := <-conn.conn.Received
		if !ok {
			break
		}
	}

	// Remove the connection from our connection set if present.
	// It will not be present if we are degraded or similar.
	store.StartTransaction()
	if _, exists := connections[conn]; exists {
		delete(connections, conn)
	}
	store.EndTransaction()
}
