package chrequest

import (
	"time"
)

import (
	"github.com/jbeshir/unanimity/config"
	"github.com/jbeshir/unanimity/shared/chrequest/chproto"
	"github.com/jbeshir/unanimity/shared/store"
)

// Performs handling of all Change Request Protocol messages,
// over all connections. Has exclusive access to forward timeout information,
// and uses requestTimeoutLock for access to request timeout information.
// Top-level function of the processing goroutine.
func process() {
	for {
		select {
		// Requesting Change Request Timeout
		case timeout := <-requestTimedOut:

			requestTimeoutLock.Lock()

			if timeout.canceled {
				break
			}

			// Remove this request timeout.
			cancelRequestTimeout(
				uint16(*timeout.request.RequestNode),
				*timeout.request.RequestId)

			// Request the change again, with double the timeout.
			addRequestTimeout(timeout.request, timeout.duration*2)

			requestTimeoutLock.Unlock()

			forward := new(chproto.ChangeForward)
			forward.Request = timeout.request
			processForward(forward)

		// Forwarded Change Forward Timeout
		case timeout := <-forwardTimedOut:
			if timeout.canceled {
				break
			}

			// Remove this forward timeout.
			cancelForwardTimeout(
				uint16(*timeout.forward.Request.RequestNode),
				*timeout.forward.Request.RequestId)

			// Try sending it somewhere else.
			processForward(timeout.forward)

		// New Change Request
		case newRequest := <-newRequestCh:

			requestTimeoutLock.Lock()

			addRequestTimeout(newRequest,
				config.CHANGE_TIMEOUT_PERIOD)

			requestTimeoutLock.Unlock()

			forward := new(chproto.ChangeForward)
			forward.Request = newRequest
			processForward(forward)

		// Received Change Forward Message
		case receivedForward := <-receivedForwardCh:
			processForward(receivedForward.forward)

		// Received Change Forward Ack Message
		case receivedAck := <-receivedAckCh:

			ack := receivedAck.ack

			// If we did not have a forwarded change forward,
			// with this requesting node and request ID,
			// and this node as the last node it was sent to,
			// discard the message.
			timeout, exists := getForwardTimeout(
				uint16(*ack.RequestNode),
				*ack.RequestId)
			if !exists {
				break
			}
			ignores := timeout.forward.Ignores
			if uint16(ignores[len(ignores)-1]) != receivedAck.node {
				break
			}

			// Remove this forward timeout.
			cancelForwardTimeout(
				uint16(*timeout.forward.Request.RequestNode),
				*timeout.forward.Request.RequestId)

		// Connection Terminated
		case terminatedConn := <-terminatedConnCh:
			node := terminatedConn.node
			conn := terminatedConn.conn

			// Remove this connection from our connection list.
			index := -1
			for i, _ := range connections[node] {
				if conn == connections[node][i] {
					index = i
					break
				}
			}
			if index == -1 {
				break
			}

			connections[node] = append(connections[node][:index],
				connections[node][index+1:]...)

		// Connection Received
		case receivedConn := <-receivedConnCh:
			node := receivedConn.node
			conn := receivedConn.conn

			// Add to our connections list.
			connections[node] = append(connections[node], conn)
		}
	}
}

// Handle a received change forward. Decides which node is responsible for it.
// Must only be called from the processing goroutine.
func processForward(forward *chproto.ChangeForward) {

	// If we are already trying to forward a change forward message with
	// the same requesting node and request ID, discard this message.
	if _, exists := getForwardTimeout(uint16(*forward.Request.RequestNode),
		*forward.Request.RequestId); exists {
		return
	}

	// Everything else in this function runs in a transaction.
	// We are read-only.
	store.StartTransaction()
	defer store.EndTransaction()

	// If this is a core node and this node stopped being leader less than
	// a Change Timeout Period ago, always add us to the ignore list.
	if config.IsCore() && !isIgnored(forward, config.Id()) {
		diff := time.Now().Sub(store.StoppedLeading())
		if diff < config.CHANGE_TIMEOUT_PERIOD {
			forward.Ignores = append(forward.Ignores,
				uint32(config.Id()))
		}
	}

	// If all core node IDs are in the forward's ignore list, discard it.
	if len(forward.Ignores) == len(config.CoreNodes()) {
		return
	}

	// Otherwise, choose a potential leader node.
	// This is O(n^2) in the number of core nodes,
	// but we don't expect to have many.
	chosenNode := uint16(0)
	_, leader := store.Proposal()
	if leader != 0 && !isIgnored(forward, leader) {
		chosenNode = leader
	} else {
		for _, node := range config.CoreNodes() {
			if !isIgnored(forward, node) {
				chosenNode = leader
				break
			}
		}
	}
	if chosenNode == 0 {
		// Shouldn't happen.
		return
	}

	// If we are the selected leader, construct an external change request,
	// and send it on our change request channel.
	if chosenNode == config.Id() {
		intRequest := forward.Request
		chrequest := new(store.ChangeRequest)
		chrequest.RequestEntity = *intRequest.RequestEntity
		chrequest.RequestNode = uint16(*intRequest.RequestNode)
		chrequest.RequestId = *intRequest.RequestId
		chrequest.Changeset = make([]store.Change,
			len(intRequest.Changeset))

		for i, ch := range intRequest.Changeset {
			chrequest.Changeset[i].TargetEntity = *ch.TargetEntity
			chrequest.Changeset[i].Key = *ch.Key
			chrequest.Changeset[i].Value = *ch.Value
		}

		for _, cb := range changeCallbacks {
			cb(chrequest)
		}

		return
	}

	// Otherwise, we send it on to the selected leader,
	// add the selected leader to the ignore list,
	// and set a timeout to retry.
	sendForward(chosenNode, forward)
	forward.Ignores = append(forward.Ignores, uint32(chosenNode))
	addForwardTimeout(forward)
}

func isIgnored(forward *chproto.ChangeForward, node uint16) bool {
	for _, ignored := range forward.Ignores {
		if uint16(ignored) == node {
			return true
		}
	}
	return false
}
