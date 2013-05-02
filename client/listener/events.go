package listener

import (
	"github.com/jbeshir/unanimity/shared/store"
)

func init() {
	store.AddDegradedCallback(handleDegraded)
}

func handleDegraded() {

	// If we become degraded, drop all clients.
	if store.Degraded() {
		connectionsLock.Lock()

		for conn, _ := range connections {
			conn.conn.Close()
		}
		connections = make(map[*userConn]bool)

		sessionsLock.Lock()
		sessions = make(map[uint64]*userConn)
		sessionsLock.Unlock()

		connectionsLock.Unlock()

	}
}
