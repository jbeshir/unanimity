package relay

import (
	"github.com/jbeshir/unanimity/shared/connect"
	"github.com/jbeshir/unanimity/shared/store"
)

func init() {
	store.AddDegradedCallback(handleDegraded)
}

func handleDegraded() {

	// If the node becomes degraded, drop all relay connections.
	if store.Degraded() {
		for _, nodeConns := range connections {
			for _, conn := range nodeConns {
				conn.Close()
			}
		}
		connections = make(map[uint16][]*connect.BaseConn)
	}
}
