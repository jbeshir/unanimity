package listener

import (
	"math/rand"
)

import (
	"github.com/jbeshir/unanimity/client/relay"
	"github.com/jbeshir/unanimity/shared/store"
)

func init() {
	// Receive relayed messages and attempt to deliver them or
	// forward them again.
	relay.AddReceivedCallback(deliver)
}

// Must be called while holding no locks and not in a transaction.
func deliver(msg *relay.UserMessage) {

	// If we have a connection to the recipient session,
	// deliver the message.
	sessionsLock.Lock()

	userConn, exists := sessions[msg.Recipient]
	if exists {
		// Try to deliver the message to this connection,
		// without blocking.
		// If its delivery channel is full, just drop it.
		select {
		case userConn.deliver <- msg:
			break
		default:
			break
		}
	}

	sessionsLock.Unlock()

	if exists {
		return
	}

	// Otherwise, decrement TTL. If it's 0, discard the message.
	msg.Ttl--
	if msg.Ttl == 0 {
		return
	}

	// Otherwise, look for another node which is connected to this session.
	store.StartTransaction()

	session := store.GetEntity(msg.Recipient)
	if session == nil || session.Value("kind") != "session" {
		// Unknown session; discard the message.
		store.EndTransaction()
		return
	}

	nodes := session.AllAttached()
	store.EndTransaction()

	if len(nodes) == 0 {
		// Shouldn't happen, sessions are transient,
		// should be deleted before reaching here.
		// Just discard the message.
		return
	}

	// Forward the message to a random one of those nodes.
	r := rand.Intn(len(nodes))
	relay.Forward(uint16(nodes[r]), msg)
}
