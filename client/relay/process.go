package relay

import (
	"time"
)

import (
	"github.com/jbeshir/unanimity/config"
	"github.com/jbeshir/unanimity/shared/connect"
	"github.com/jbeshir/unanimity/shared/store"
)

// Access only from inside a transaction.
var connections map[uint16][]*connect.BaseConn

func init() {
	connections = make(map[uint16][]*connect.BaseConn)
}

func process() {

	// Try to make an outgoing connection to all other client nodes,
	// if we're not in a degraded state.
	store.StartTransaction()
	if !store.Degraded() {

		for _, node := range config.ClientNodes() {
			if node == config.Id() {
				continue
			}

			conn, err := connect.Dial(connect.RELAY_PROTOCOL, node)
			if err != nil {
				// No luck connecting.
				continue
			}

			connections[node] = append(connections[node], conn)
			go handleConn(node, conn)
		}
	}
	store.EndTransaction()

	// Retry connections once per config.CHANGE_TIMEOUT_PERIOD
	// Largely arbitrary.
	reconnectTicker := time.Tick(config.CHANGE_TIMEOUT_PERIOD)

	for {
		select {

		// Connection retry tick.
		// If not degraded, try to make an outgoing connection to any
		// client node that we do not have at least one connection to.
		case <-reconnectTicker:

			store.StartTransaction()

			// Do not attempt to make connections while degraded.
			if store.Degraded() {
				store.EndTransaction()
				break
			}

			for _, node := range config.ClientNodes() {
				if node == config.Id() {
					continue
				}
				if len(connections[node]) > 0 {
					continue
				}

				conn, err := connect.Dial(
					connect.RELAY_PROTOCOL, node)

				if err != nil {
					// No luck connecting.
					continue
				}

				connections[node] =
					append(connections[node], conn)
				go handleConn(node, conn)
			}

			store.EndTransaction()

		// New received connection.
		case receivedConn := <-receivedConnCh:
			node := receivedConn.node
			conn := receivedConn.conn

			store.StartTransaction()

			// If we are degraded, reject the connection.
			if store.Degraded() {
				conn.Close()
				store.EndTransaction()
				break
			}

			// Add to our connections.
			connections[node] = append(connections[node], conn)

			store.EndTransaction()

		// Terminated connection.
		case receivedConn := <-terminatedConnCh:
			node := receivedConn.node
			conn := receivedConn.conn

			store.StartTransaction()

			// Remove this connection from our connection list.
			index := -1
			for i, _ := range connections[node] {
				if conn == connections[node][i] {
					index = i
					break
				}
			}
			if index != -1 {
				connections[node] =
					append(connections[node][:index],
						connections[node][index+1:]...)
			}

			store.EndTransaction()
		}
	}
}
