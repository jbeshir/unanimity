package consensus

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

// Top-level function of the processing goroutine.
func process() {
	for {
		select {

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

func appendInst(list *[]*coproto.Instruction, slot uint64,
	inst *store.InstructionValue) {

	instProposal, instLeader := inst.Proposal()

	internalInst := new(coproto.Instruction)
	internalInst.Slot = new(uint64)
	internalInst.Proposal = new(uint64)
	internalInst.Leader = new(uint32)
	*internalInst.Slot = slot
	*internalInst.Proposal = instProposal
	*internalInst.Leader = uint32(instLeader)

	internalInst.Request = makeIntChangeRequest(inst.ChangeRequest())

	*list = append(*list, internalInst)	
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
