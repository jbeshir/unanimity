package follow

import (
	"code.google.com/p/goprotobuf/proto"
)

import (
	"github.com/jbeshir/unanimity/shared/connect/baseproto"
	"github.com/jbeshir/unanimity/shared/follow/fproto"
	"github.com/jbeshir/unanimity/shared/store"
)

func init() {
	store.AddAppliedCallback(handleApplied)
	store.AddChosenCallback(handleChosen)
	store.AddDegradedCallback(handleDegraded)
	store.AddProposalCallback(handleProposalChange)
}

func handleApplied(slot uint64, idempotentChanges []store.Change) {

	relativeSlot := int(slot - store.InstructionStart())
	slots := store.InstructionSlots()

	origReq := slots[relativeSlot][0].ChangeRequest()
	chrequest := new(store.ChangeRequest)
	chrequest.RequestEntity = origReq.RequestEntity
	chrequest.RequestNode = origReq.RequestNode
	chrequest.RequestId = origReq.RequestId
	chrequest.Changeset = idempotentChanges

	connectionsLock.Lock()

	for _, conn := range connections {

		conn.lock.Lock()

		if conn.sendingBurst {
			sendInstructionData(conn, slot, chrequest)
		}

		conn.lock.Unlock()
	}

	connectionsLock.Unlock()

}

func handleChosen(slot uint64) {

	var msg fproto.InstructionChosen
	msg.Slot = new(uint64)
	*msg.Slot = slot
	msgBuf, err := proto.Marshal(&msg)
	if err != nil {
		panic("generated bad instruction chosen message")
	}

	var baseMsg baseproto.Message
	baseMsg.MsgType = new(uint32)
	*baseMsg.MsgType = 7
	baseMsg.Content = msgBuf

	connectionsLock.Lock()

	for _, conn := range connections {

		conn.lock.Lock()

		if !conn.sendingBurst {
			conn.conn.Send(&baseMsg)
		}

		if timer, exists := conn.offerTimers[slot]; exists {
			timer.Stop()
			delete(conn.offerTimers, slot)
		}

		conn.lock.Unlock()
	}

	connectionsLock.Unlock()
}

func handleDegraded() {

	// If we become degraded, close all follow connections
	// aside those receiving bursts.
	if store.Degraded() {

		connectionsLock.Lock()

		for _, conn := range connections {

			conn.lock.Lock()

			if !conn.receivingBurst {
				conn.Close()
			}

			conn.lock.Unlock()
		}

		connectionsLock.Unlock()
	}
}

func handleProposalChange() {
	proposal, leader := store.Proposal()

	var msg fproto.Leader
	msg.Proposal = new(uint64)
	msg.Leader = new(uint32)
	*msg.Proposal = proposal
	*msg.Leader = uint32(leader)
	msgBuf, err := proto.Marshal(&msg)
	if err != nil {
		panic("generated bad leader update message")
	}

	var baseMsg baseproto.Message
	baseMsg.MsgType = new(uint32)
	*baseMsg.MsgType = 11
	baseMsg.Content = msgBuf

	connectionsLock.Lock()

	for _, conn := range connections {
		conn.conn.Send(&baseMsg)
	}

	connectionsLock.Unlock()
}
