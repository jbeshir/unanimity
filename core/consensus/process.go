package consensus

import (
	"time"
)

import (
	"code.google.com/p/goprotobuf/proto"
)

import (
	"github.com/jbeshir/unanimity/config"
	"github.com/jbeshir/unanimity/core/consensus/coproto"
	"github.com/jbeshir/unanimity/shared/connect"
	"github.com/jbeshir/unanimity/shared/store"
)

var connections map[uint16][]*connect.BaseConn

// Non-nil means we're attempting to become leader.
// Can only be accessed inside a transaction.
var receivedPromises map[uint16]*coproto.Promise

// Can only be accessed inside a transaction.
var waitingRequests []*coproto.ChangeRequest

// Can only be accessed inside a transaction.
var amLeader bool
var nextProposalSlot uint64 // Set on becoming leader.

// Top-level function of the processing goroutine.
func process() {

	connections = make(map[uint16][]*connect.BaseConn)

	// On startup, make an outgoing connection attempt to all other
	// core nodes, before continuing.
	for _, node := range config.CoreNodes() {
		if node == config.Id() {
			continue
		}

		conn, err := connect.Dial(connect.CONSENSUS_PROTOCOL, node)
		if err != nil {
			// Can't reach the other node.
			continue
		}

		connections[node] = append(connections[node], conn)
	}

	// Retry connections once per config.CHANGE_TIMEOUT_PERIOD.
	// Largely arbitrary.
	reconnectTicker := time.Tick(config.CHANGE_TIMEOUT_PERIOD)

	for {
		select {

		// Connection retry tick.
		// We should try to make an outgoing connection to any node
		// that we do not have at least one connection to.
		case <-reconnectTicker:
			for _, node := range config.CoreNodes() {
				if node == config.Id() {
					continue
				}
				if len(connections[node]) > 0 {
					continue
				}

				conn, err := connect.Dial(
					connect.CONSENSUS_PROTOCOL, node)
				if err != nil {
					// Can't reach the other node.
					continue
				}

				connections[node] =
					append(connections[node], conn)
			}

		// New change request, for us to propose as leader.
		case req := <-newChangeCh:

			store.StartTransaction()

			if !amLeader && receivedPromises != nil {
				waitingRequests = append(waitingRequests, makeIntChangeRequest(req))
			} else if !amLeader {
				waitingRequests = append(waitingRequests,
					makeIntChangeRequest(req))

				// Start attempting to be leader.
				m := make(map[uint16]*coproto.Promise)
				receivedPromises = m

				proposal, _ := store.Proposal()
				proposal++
				store.SetProposal(proposal, config.Id())
				firstUn := store.InstructionFirstUnapplied()

				// Send prepare messages to all other nodes.
				var prepare coproto.Prepare
				prepare.Proposal = new(uint64)
				prepare.FirstUnapplied = new(uint64)
				*prepare.Proposal = proposal
				*prepare.FirstUnapplied = firstUn

				for _, node := range config.CoreNodes() {
					if node == config.Id() {
						continue
					}

					if len(connections[node]) != 0 {
						c := connections[node][0]
						c.SendProto(2, &prepare)
					}
				}

				// Behave as if we got a promise message
				// from ourselves.
				var promise coproto.Promise
				promise.Proposal = prepare.Proposal
				promise.PrevProposal = promise.Proposal
				promise.Leader = new(uint32)
				*promise.Leader = uint32(config.Id())
				promise.PrevLeader = promise.Leader
				addPromise(config.Id(), &promise)
			} else {
				newSlot := nextProposalSlot
				nextProposalSlot++

				addProposalTimeout(newSlot, req)

				proposal, leader := store.Proposal()
				inst := makeInst(newSlot, proposal, leader,
					req)

				// Send accept messages to all other core nodes.
				var accept coproto.Accept
				accept.Proposal = new(uint64)
				*accept.Proposal = proposal
				accept.Instruction = inst

				for _, node := range config.CoreNodes() {
					if node == config.Id() {
						continue
					}

					if len(connections[node]) != 0 {
						c := connections[node][0]
						c.SendProto(4, &accept)
					}
				}

				// Behave as if we got an accepted
				// message from ourselves.
				var accepted coproto.Accepted
				accepted.Proposal = accept.Proposal
				accepted.Leader = new(uint32)
				*accepted.Leader = uint32(leader)
				accepted.Instruction = inst
				addAccepted(config.Id(), &accepted)
			}

			store.EndTransaction()

		// Leadership attempt timed out.
		case timedOutProposal := <-leaderTimeoutCh:

			store.StartTransaction()

			proposal, leader := store.Proposal()

			// If the proposal has changed since that
			// leadership attempt, ignore it.
			if leader != config.Id() {
				return
			}
			if proposal != timedOutProposal {
				return
			}

			// If we successfully became leader, ignore it.
			if amLeader {
				return
			}

			// Otherwise, stop our attempt to become leader.
			stopBeingLeader()

			store.EndTransaction()

		// Proposal timed out.
		case timeout := <-proposalTimeoutCh:

			store.StartTransaction()

			// If this timeout was not canceled, a proposal failed.
			// We stop being leader.
			if proposalTimeouts[timeout.slot] == timeout {
				stopBeingLeader()
			}

			store.EndTransaction()

		// New received connection.
		case receivedConn := <-receivedConnCh:
			node := receivedConn.node
			conn := receivedConn.conn

			connections[node] = append(connections[node], conn)

		// Received message.
		case recvMsg := <-receivedMsgCh:
			node := recvMsg.node
			conn := recvMsg.conn
			msg := recvMsg.msg
			switch *msg.MsgType {
			case 2:
				processPrepare(node, conn, msg.Content)
			case 3:
				processPromise(node, conn, msg.Content)
			case 4:
				processAccept(node, conn, msg.Content)
			default:
				// Unknown message.
				conn.Close()
			}

		// Terminate received connection.
		case terminatedConn := <-terminatedConnCh:
			node := terminatedConn.node
			conn := terminatedConn.conn

			for i, other := range connections[node] {
				if other != conn {
					continue
				}

				conns := connections[node]
				conns = append(conns[:i], conns[i+1:]...)
				connections[node] = conns
				break
			}
		}
	}
}

// Must be called from the processing goroutine.
func processPrepare(node uint16, conn *connect.BaseConn, content []byte) {
	var msg coproto.Prepare
	if err := proto.Unmarshal(content, &msg); err != nil {
		conn.Close()
		return
	}

	store.StartTransaction()
	defer store.EndTransaction()

	newProposal, newLeader := *msg.Proposal, node
	proposal, leader := store.Proposal()
	if store.CompareProposals(newProposal, newLeader, proposal, leader) {

		// Create a promise message to send back.
		var promise coproto.Promise
		promise.Proposal = new(uint64)
		promise.Leader = new(uint32)
		promise.PrevProposal = new(uint64)
		promise.PrevLeader = new(uint32)
		*promise.Proposal = newProposal
		*promise.Leader = uint32(newLeader)
		*promise.PrevProposal = proposal
		*promise.PrevLeader = uint32(leader)

		// Add all the instructions we've previously accepted or chosen.
		slots := store.InstructionSlots()
		theirFirstUnapplied := *msg.FirstUnapplied
		ourStart := store.InstructionStart()
		relativeSlot := int(theirFirstUnapplied - ourStart)
		if relativeSlot < 0 {
			relativeSlot = 0
		}
		var accepted []*coproto.Instruction
		for ; relativeSlot < len(slots); relativeSlot++ {
			slot := slots[relativeSlot]
			slotNum := ourStart + uint64(relativeSlot)
			for i, _ := range slot {
				if slot[i].IsChosen() {
					appendInst(&accepted, slotNum, slot[i])
					break
				}

				weAccepted := false
				for _, node := range slot[i].Accepted() {
					if node == config.Id() {
						weAccepted = true
						break
					}
				}

				if weAccepted {
					appendInst(&accepted, slotNum, slot[i])
					break
				}
			}
		}

		// Accept the other node as our new leader.
		store.SetProposal(newProposal, newLeader)
	} else {
		var nack coproto.Nack
		nack.Proposal = new(uint64)
		nack.Leader = new(uint32)
		*nack.Proposal = proposal
		*nack.Leader = uint32(leader)
		conn.SendProto(6, &nack)
	}
}

// Must be called from the processing goroutine.
func processPromise(node uint16, conn *connect.BaseConn, content []byte) {
	var msg coproto.Promise
	if err := proto.Unmarshal(content, &msg); err != nil {
		conn.Close()
		return
	}

	store.StartTransaction()
	defer store.EndTransaction()

	if receivedPromises == nil {
		// Not attempting to become leader.
		return
	}

	proposal, leader := store.Proposal()
	if proposal != *msg.Proposal || leader != uint16(*msg.Leader) {
		return
	}
	if receivedPromises[node] != nil {
		// Protocol violation; shouldn't get duplicate promises.
		return
	}

	addPromise(node, &msg)
}

// Must be called from the processing goroutine.
func processAccept(node uint16, conn *connect.BaseConn, content []byte) {
	var msg coproto.Accept
	if err := proto.Unmarshal(content, &msg); err != nil {
		conn.Close()
		return
	}

	store.StartTransaction()
	defer store.EndTransaction()

	proposal, leader := store.Proposal()
	msgProposal, msgLeader := *msg.Proposal, node
	if proposal != msgProposal || leader != msgLeader {
		// Send a nack message and return,
		// if this accept relates to an earlier proposal.
		if store.CompareProposals(proposal, leader, msgProposal,
			msgLeader) {

			var nack coproto.Nack
			nack.Proposal = new(uint64)
			nack.Leader = new(uint32)
			*nack.Proposal = proposal
			*nack.Leader = uint32(leader)
			conn.SendProto(6, &nack)

			return
		}

		store.SetProposal(msgProposal, msgLeader)
	}

	// Send an accepted message to all other core nodes reachable.
	var accepted coproto.Accepted
	accepted.Proposal = msg.Proposal
	accepted.Leader = new(uint32)
	*accepted.Leader = uint32(msgLeader)
	accepted.Instruction = msg.Instruction

	// We must do this before sending accepted messages.
	if !addAccepted(config.Id(), &accepted) {
		// Required too large an instruction slots slice.
		// We have NOT accepted this accept message,
		// and must NOT send accepted messages.
		return
	}

	for _, node := range config.CoreNodes() {
		if node == config.Id() {
			continue
		}

		if len(connections[node]) > 0 {
			connections[node][0].SendProto(5, &accepted)
		}
	}

}

// Must be called from the processing goroutine.
func processAccepted(node uint16, conn *connect.BaseConn, content []byte) {
	var msg coproto.Accepted
	if err := proto.Unmarshal(content, &msg); err != nil {
		conn.Close()
		return
	}

	store.StartTransaction()
	defer store.EndTransaction()

	addAccepted(node, &msg)
}

// Must be called from the processing goroutine.
func processNack(node uint16, conn *connect.BaseConn, content []byte) {
	var msg coproto.Nack
	if err := proto.Unmarshal(content, &msg); err != nil {
		conn.Close()
		return
	}

	store.StartTransaction()
	defer store.EndTransaction()

	// If we don't consider ourselves the leader, discard.
	if !amLeader {
		return
	}

	msgProposal, msgLeader := *msg.Proposal, uint16(*msg.Leader)
	proposal, leader := store.Proposal()
	if msgProposal == proposal && msgLeader == leader {
		return
	}
	if store.CompareProposals(msgProposal, msgLeader, proposal, leader) {
		stopBeingLeader()
		store.SetProposal(msgProposal, msgLeader)
	}
}

// Stops this node from considering itself the leader,
// and aborts any attempt to be leader.
// Does not change our current proposal number and leader node ID.
// To do that, call store.SetProposal after calling this.
// Must be called from the processing goroutine, inside a transacton.
func stopBeingLeader() {
	amLeader = false
	store.StopLeading()

	for _, timeout := range proposalTimeouts {
		timeout.timer.Stop()
	}
	proposalTimeouts = make(map[uint64]*proposalTimeout)

	waitingRequests = nil
	receivedPromises = nil
}

// Must be called from the processing goroutine, inside a transaction.
func addPromise(node uint16, msg *coproto.Promise) {
	receivedPromises[node] = msg

	// TODO: BECOME LEADER
}

// Must be called from the processing goroutine, inside a transaction.
// Returns false if it discarded the accepted message because of it being
// too far in our future. Any other reason for discarding the message,
// as well as the accept succeeding, will return true.
func addAccepted(node uint16, accepted *coproto.Accepted) bool {

	slots := store.InstructionSlots()

	msgProposal, msgLeader := *accepted.Proposal, uint16(*accepted.Leader)
	newReq := accepted.Instruction.Request
	slot := *accepted.Instruction.Slot
	ourStart := store.InstructionStart()
	relativeSlot := int(slot - ourStart)

	// If we have already chosen an instruction in this slot,
	// we can ignore subsequent Accepted messages for it.
	if len(slots[relativeSlot]) == 1 && slots[relativeSlot][0].IsChosen() {
		return true
	}

	// Returns if it successfully finds an existing matching value.
	for _, value := range slots[relativeSlot] {
		chreq := value.ChangeRequest()
		if chreq.RequestNode != uint16(*newReq.RequestNode) {
			continue
		}
		if chreq.RequestId != *newReq.RequestId {
			continue
		}

		value.Accept(node, msgProposal, msgLeader)
		return true
	}

	chreq := makeExtChangeRequest(newReq)
	newInst := store.AddInstructionValue(slot, chreq)
	if newInst == nil {
		return false
	}
	newInst.Accept(node, msgProposal, msgLeader)

	return true
}

func appendInst(list *[]*coproto.Instruction, slot uint64,
	inst *store.InstructionValue) {

	instProposal, instLeader := inst.Proposal()
	internalInst := makeInst(slot, instProposal, instLeader,
		inst.ChangeRequest())

	*list = append(*list, internalInst)
}

func makeInst(slot uint64, proposal uint64, leader uint16,
	req *store.ChangeRequest) *coproto.Instruction {

	internalInst := new(coproto.Instruction)
	internalInst.Slot = new(uint64)
	internalInst.Proposal = new(uint64)
	internalInst.Leader = new(uint32)
	*internalInst.Slot = slot
	*internalInst.Proposal = proposal
	*internalInst.Leader = uint32(leader)

	internalInst.Request = makeIntChangeRequest(req)

	return internalInst
}

func makeIntChangeRequest(req *store.ChangeRequest) *coproto.ChangeRequest {

	intReq := new(coproto.ChangeRequest)
	intReq.RequestEntity = new(uint64)
	intReq.RequestNode = new(uint32)
	intReq.RequestId = new(uint64)
	*intReq.RequestEntity = req.RequestEntity
	*intReq.RequestNode = uint32(req.RequestNode)
	*intReq.RequestId = req.RequestId

	intReq.Changeset = make([]*coproto.Change, len(req.Changeset))
	for i := range req.Changeset {
		intCh := new(coproto.Change)
		intCh.TargetEntity = new(uint64)
		intCh.Key = new(string)
		intCh.Value = new(string)
		*intCh.TargetEntity = req.Changeset[i].TargetEntity
		*intCh.Key = req.Changeset[i].Key
		*intCh.Value = req.Changeset[i].Value
		intReq.Changeset[i] = intCh
	}

	return intReq
}

func makeExtChangeRequest(intReq *coproto.ChangeRequest) *store.ChangeRequest {

	req := new(store.ChangeRequest)
	req.RequestEntity = *intReq.RequestEntity
	req.RequestNode = uint16(*intReq.RequestNode)
	req.RequestId = *intReq.RequestId

	req.Changeset = make([]store.Change, len(intReq.Changeset))
	for i, intCh := range intReq.Changeset {
		req.Changeset[i].TargetEntity = *intCh.TargetEntity
		req.Changeset[i].Key = *intCh.Key
		req.Changeset[i].Value = *intCh.Value
	}

	return req
}
