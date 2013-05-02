package follow

import (
	"math/rand"
	"sync"
	"time"
)

import (
	"github.com/jbeshir/unanimity/config"
	"github.com/jbeshir/unanimity/shared/connect"
	"github.com/jbeshir/unanimity/shared/follow/fproto"
	"github.com/jbeshir/unanimity/shared/store"
)

var connectionsLock sync.Mutex
var connections map[uint16]*followConn

var tryOutgoingCh <-chan time.Time
var processRand *rand.Rand

func init() {
	connections = make(map[uint16]*followConn)

	newTryOutgoingCh := make(chan time.Time, 1)
	newTryOutgoingCh <- time.Now()
	tryOutgoingCh = newTryOutgoingCh

	unixTime := time.Now().Unix()
	processRand = rand.New(rand.NewSource(unixTime))
}

func process() {

	for {
		select {

		// Try to make any needed outgoing connections.
		case <-tryOutgoingCh:
			store.StartTransaction()
			connectionsLock.Lock()

			coreNodes := config.CoreNodes()
			if !store.Degraded() {
				if config.IsCore() {
					for _, node := range coreNodes {
						if node == config.Id() {
							continue
						}

						if connections[node] == nil {
							go tryOutgoing(node)
						}
					}
				} else {

					// Settle for less than 2 core nodes,
					// if there *aren't* 2 core nodes.
					targetConns := 2
					if targetConns > len(coreNodes) {
						targetConns = len(coreNodes)
					}

					newOutgoing := targetConns - len(connections)
					for newOutgoing > 0 {
						r := processRand.Intn(len(coreNodes))
						node := coreNodes[r]
						if connections[node] == nil {
							go tryOutgoing(node)
							newOutgoing--
						}
					}
				}
			} else {
				if len(connections) == 0 {
					r := processRand.Intn(len(coreNodes))
					node := coreNodes[r]
					if connections[node] == nil {
						go tryOutgoing(node)
					}
				}
			}

			connectionsLock.Unlock()
			store.EndTransaction()

			// After a random time, check again.
			randDur := time.Duration(processRand.Intn(19)+1) * time.Second
			tryOutgoingCh = time.NewTimer(randDur).C

		// New received connection.
		case conn := <-receivedConnCh:

			conn.offerTimers = make(map[uint64]*time.Timer)

			store.StartTransaction()
			connectionsLock.Lock()

			// If we are degraded, reject the connection,
			// unless we have no other connection,
			// and it is outbound.
			if store.Degraded() &&
				!(len(connections) == 0 && conn.outgoing) {

				conn.lock.Lock()
				conn.Close()
				conn.lock.Unlock()
				connectionsLock.Unlock()
				store.EndTransaction()
				break
			}

			// If we have an existing connection to this node,
			// close it.
			if connections[conn.node] != nil {
				other := connections[conn.node]

				other.lock.Lock()

				other.Close()
				if other.firstUnappliedTimer != nil {
					other.firstUnappliedTimer.Stop()
				}

				other.lock.Unlock()
			}

			// Add to our connections.
			connections[conn.node] = conn

			// Send initial position and leader messages.
			posMsg := new(fproto.Position)
			posMsg.FirstUnapplied = new(uint64)
			posMsg.Degraded = new(bool)
			*posMsg.FirstUnapplied =
				store.InstructionFirstUnapplied()
			*posMsg.Degraded = store.Degraded()
			conn.conn.SendProto(2, posMsg)

			proposal, leader := store.Proposal()
			leaderMsg := new(fproto.Leader)
			leaderMsg.Proposal = new(uint64)
			leaderMsg.Leader = new(uint32)
			*leaderMsg.Proposal = proposal
			*leaderMsg.Leader = uint32(leader)
			conn.conn.SendProto(11, leaderMsg)

			connectionsLock.Unlock()
			store.EndTransaction()

			// Start timer for sending first unapplied
			// instruction updates.
			if config.IsCore() && conn.node <= 0x2000 {
				conn.lock.Lock()

				conn.firstUnappliedTimer = time.AfterFunc(
					firstUnappliedTimerDuration,
					func() { conn.firstUnappliedTimeout() })

				conn.lock.Unlock()
			}

			// Start handling received messages from the connection.
			go handleConn(conn)

		// Terminated connection.
		case conn := <-terminatedConnCh:

			connectionsLock.Lock()

			if connections[conn.node] == conn {
				delete(connections, conn.node)

				conn.lock.Lock()

				conn.closed = true
				if conn.firstUnappliedTimer != nil {
					conn.firstUnappliedTimer.Stop()
				}

				conn.lock.Unlock()
			}

			connectionsLock.Unlock()
		}
	}
}

// Try to make an outgoing connection to the given node ID.
func tryOutgoing(node uint16) {

	conn, err := connect.Dial(connect.FOLLOW_PROTOCOL, node)
	if err != nil {
		// No luck connecting.
		return
	}

	// Send the new outgoing connection to the processing goroutine.
	followConn := new(followConn)
	followConn.node = node
	followConn.conn = conn
	followConn.outgoing = true
	receivedConnCh <- followConn

}
