package follow

import (
	"log"
	"time"
)

import (
	"code.google.com/p/goprotobuf/proto"
)

import (
	"github.com/jbeshir/unanimity/config"
	"github.com/jbeshir/unanimity/shared/connect/baseproto"
	"github.com/jbeshir/unanimity/shared/follow/fproto"
	"github.com/jbeshir/unanimity/shared/store"
)

var terminatedConnCh chan *followConn

func init() {
	terminatedConnCh = make(chan *followConn)
}

func handleConn(f *followConn) {

	for {
		msg, ok := <-f.conn.Received
		if !ok {
			break
		}

		switch *msg.MsgType {
		case 2:
			handlePosition(f, msg.Content)
		case 3:
			handleBursting(f, msg.Content)
		case 4:
			handleGlobalProperty(f, msg.Content)
		case 5:
			handleEntityProperty(f, msg.Content)
		case 6:
			handleBurstDone(f, msg.Content)
		case 7:
			handleInstructionChosen(f, msg.Content)
		case 8:
			handleInstructionRequest(f, msg.Content)
		case 9:
			handleInstructionData(f, msg.Content)
		case 10:
			handleFirstUnapplied(f, msg.Content)
		case 11:
			handleLeader(f, msg.Content)
		default:
			f.lock.Lock()
			f.Close()
			f.lock.Unlock()
		}
	}

	terminatedConnCh <- f
}

func handlePosition(f *followConn, content []byte) {
	store.StartTransaction()
	defer store.EndTransaction()
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.closed {
		return
	}

	var msg fproto.Position
	if err := proto.Unmarshal(content, &msg); err != nil {
		f.Close()
		return
	}

	if config.IsCore() && f.node <= 0x2000 {
		store.SetNodeFirstUnapplied(f.node, *msg.FirstUnapplied)
	}

	if store.Degraded() && *msg.Degraded {
		f.Close()
		return
	}

	start := store.InstructionStart()
	if *msg.FirstUnapplied < start {

		f.sendingBurst = true
		burstMsg := new(baseproto.Message)
		burstMsg.MsgType = new(uint32)
		burstMsg.Content = []byte{}
		*burstMsg.MsgType = 3
		f.conn.Send(burstMsg)

		go sendBurst(f)

		return

	} else if *msg.FirstUnapplied <= store.InstructionFirstUnapplied() &&
		*msg.Degraded {

		f.sendingBurst = true
		burstMsg := new(baseproto.Message)
		burstMsg.MsgType = new(uint32)
		*burstMsg.MsgType = 3
		f.conn.Send(burstMsg)

		go sendBurst(f)

		return

	} else if *msg.Degraded {
		f.Close()
		return
	}

	// Send all chosen instructions above the first unapplied point.
	instructions := store.InstructionSlots()
	relativeSlot := int(*msg.FirstUnapplied - store.InstructionStart())
	for ; relativeSlot < len(instructions); relativeSlot++ {
		slot := store.InstructionStart() + uint64(relativeSlot)
		slotValues := instructions[relativeSlot]
		if len(slotValues) != 1 || !slotValues[0].IsChosen() {
			continue
		}

		// Convert the change request to our internal format.
		sendInstructionData(f, slot, slotValues[0].ChangeRequest())
	}
}

func handleBursting(f *followConn, content []byte) {
	store.StartTransaction()
	defer store.EndTransaction()
	f.lock.Lock()

	if f.closed {
		f.lock.Unlock()
		return
	}

	f.receivingBurst = true

	f.lock.Unlock()

	store.Degrade()
}

func handleGlobalProperty(f *followConn, content []byte) {
	store.StartTransaction()
	defer store.EndTransaction()
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.closed {
		return
	}

	var msg fproto.GlobalProperty
	if err := proto.Unmarshal(content, &msg); err != nil {
		f.Close()
		return
	}

	if !f.receivingBurst {
		f.Close()
		return
	}

	store.BurstGlobal(*msg.Key, *msg.Value)
}

func handleEntityProperty(f *followConn, content []byte) {
	store.StartTransaction()
	defer store.EndTransaction()
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.closed {
		return
	}

	var msg fproto.EntityProperty
	if err := proto.Unmarshal(content, &msg); err != nil {
		f.Close()
		return
	}

	if !f.receivingBurst {
		f.Close()
		return
	}

	store.BurstEntity(*msg.Entity, *msg.Key, *msg.Value)
}

func handleBurstDone(f *followConn, content []byte) {
	store.StartTransaction()
	defer store.EndTransaction()
	f.lock.Lock()

	if f.closed {
		f.lock.Unlock()
		return
	}

	var msg fproto.BurstDone
	if err := proto.Unmarshal(content, &msg); err != nil {
		f.Close()
		f.lock.Unlock()
		return
	}

	if !f.receivingBurst {
		f.Close()
		f.lock.Unlock()
		return
	}
	f.receivingBurst = false

	wasWaiting := f.waiting
	f.waiting = nil

	// We need to unlock before we start mutating the store,
	// due to callbacks from the store package to elsewhere.
	f.lock.Unlock()

	chrequests := make([]store.ChangeRequest, len(wasWaiting))
	for i, data := range wasWaiting {
		req := data.Request
		chrequests[i].RequestEntity = *req.RequestEntity
		chrequests[i].RequestNode = uint16(*req.RequestNode)
		chrequests[i].RequestId = *req.RequestId
		chrequests[i].Changeset =
			make([]store.Change, len(req.Changeset))

		chset := chrequests[i].Changeset
		for j, ch := range req.Changeset {
			chset[j].TargetEntity = *ch.TargetEntity
			chset[j].Key = *ch.Key
			chset[j].Value = *ch.Value
		}
	}
	store.EndBurst(*msg.FirstUnapplied, chrequests)
}

func handleInstructionChosen(f *followConn, content []byte) {
	store.StartTransaction()
	defer store.EndTransaction()
	f.lock.Lock()
	defer f.lock.Unlock()

	var msg fproto.InstructionChosen
	if err := proto.Unmarshal(content, &msg); err != nil {
		f.Close()
		return
	}

	if f.closed {
		return
	}

	relativeSlot := int(*msg.Slot - store.InstructionStart())
	if relativeSlot < 0 {
		return
	}
	instructions := store.InstructionSlots()
	if relativeSlot < len(instructions) {
		slot := instructions[relativeSlot]
		if len(slot) == 1 && slot[0].IsChosen() {
			return
		}
	}

	if !config.IsCore() {
		// TODO: Should only do this on one follow connection.
		var intReq fproto.InstructionRequest
		intReq.Slot = msg.Slot
		f.conn.SendProto(8, &intReq)
	}

	timeout := config.ROUND_TRIP_TIMEOUT_PERIOD
	f.offerTimers[*msg.Slot] = time.AfterFunc(
		timeout, func() { f.offerTimeout(*msg.Slot, timeout) })
}

func handleInstructionRequest(f *followConn, content []byte) {
	store.StartTransaction()
	defer store.EndTransaction()
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.closed {
		return
	}

	var msg fproto.InstructionRequest
	if err := proto.Unmarshal(content, &msg); err != nil {
		f.Close()
		return
	}

	relativeSlot := int(*msg.Slot - store.InstructionStart())
	if relativeSlot < 0 {
		f.Close()
		return
	}
	instructions := store.InstructionSlots()
	if relativeSlot >= len(instructions) {
		f.Close()
		return
	}
	slot := instructions[relativeSlot]
	if len(slot) != 1 || !slot[0].IsChosen() {
		f.Close()
		return
	}

	// Convert the change request to our internal format.
	sendInstructionData(f, *msg.Slot, slot[0].ChangeRequest())
}

func handleInstructionData(f *followConn, content []byte) {
	store.StartTransaction()
	defer store.EndTransaction()
	f.lock.Lock()

	if f.closed {
		f.lock.Unlock()
		return
	}

	var msg fproto.InstructionData
	if err := proto.Unmarshal(content, &msg); err != nil {
		f.Close()
		f.lock.Unlock()
		return
	}

	if f.receivingBurst {
		f.waiting = append(f.waiting, &msg)
		f.lock.Unlock()
		return
	}

	// We need to unlock before we start mutating the store,
	// due to callbacks from the store package to elsewhere.
	f.lock.Unlock()

	// If we have a chosen instruction in this slot already,
	// or it is prior to our instruction start number,
	// discard this message.
	relativeSlot := int(*msg.Slot - store.InstructionStart())
	if relativeSlot < 0 {
		return
	}

	instructions := store.InstructionSlots()
	if relativeSlot < len(instructions) &&
		len(instructions[relativeSlot]) == 1 &&
		instructions[relativeSlot][0].IsChosen() {

		return
	}

	// Construct a store.ChangeRequest from our
	// internal ChangeRequest.
	chrequest := new(store.ChangeRequest)
	chrequest.RequestEntity = *msg.Request.RequestEntity
	chrequest.RequestNode = uint16(*msg.Request.RequestNode)
	chrequest.RequestId = *msg.Request.RequestId
	chrequest.Changeset = make([]store.Change,
		len(msg.Request.Changeset))

	for i, ch := range msg.Request.Changeset {
		chrequest.Changeset[i].TargetEntity = *ch.TargetEntity
		chrequest.Changeset[i].Key = *ch.Key
		chrequest.Changeset[i].Value = *ch.Value
	}

	// Add instruction value and immediately choose it.
	store.AddInstructionValue(*msg.Slot, chrequest).Choose()
}

func handleFirstUnapplied(f *followConn, content []byte) {

	// Client nodes ignore this message, and shouldn't send it.
	if !config.IsCore() || f.node > 0x2000 {
		return
	}

	store.StartTransaction()
	defer store.EndTransaction()
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.closed {
		return
	}

	var msg fproto.FirstUnapplied
	if err := proto.Unmarshal(content, &msg); err != nil {
		f.Close()
		return
	}

	// Can't trigger callbacks from the store package to elsewhere,
	// safe to call while holding f's lock.
	store.SetNodeFirstUnapplied(f.node, *msg.FirstUnapplied)
}

func handleLeader(f *followConn, content []byte) {
	var msg fproto.Leader
	if err := proto.Unmarshal(content, &msg); err != nil {
		f.lock.Lock()
		f.Close()
		f.lock.Unlock()
		return
	}

	store.StartTransaction()
	defer store.EndTransaction()

	proposal, leader := store.Proposal()
	msgProposal, msgLeader := *msg.Proposal, uint16(*msg.Leader)
	if store.CompareProposals(msgProposal, msgLeader, proposal, leader) {
		store.SetProposal(msgProposal, msgLeader)
	}
}

// Sends a burst to the given connection.
// The Bursting message should already have been sent,
// and f.sendingBurst set to true, before calling this asynchronously.
func sendBurst(f *followConn) {

	log.Print("shared/follow: sending burst to ", f.node)

	globalProp := new(fproto.GlobalProperty)
	globalProp.Key = new(string)
	globalProp.Value = new(string)

	entityProp := new(fproto.EntityProperty)
	entityProp.Entity = new(uint64)
	entityProp.Key = new(string)
	entityProp.Value = new(string)

	aborted := false
	store.Burst(func(key, value string) bool {
		*globalProp.Key = key
		*globalProp.Value = value

		if !f.conn.SendProto(4, globalProp) {
			aborted = true
		}
		return !aborted
	}, func(entity uint64, key, value string) bool {
		*entityProp.Entity = entity
		*entityProp.Key = key
		*entityProp.Value = value

		if !f.conn.SendProto(5, entityProp) {
			aborted = true
		}
		return !aborted
	})

	if aborted {
		return
	}

	store.StartTransaction()
	defer store.EndTransaction()
	f.lock.Lock()
	defer f.lock.Unlock()

	f.sendingBurst = false

	burstDone := new(fproto.BurstDone)
	burstDone.FirstUnapplied = new(uint64)
	*burstDone.FirstUnapplied = store.InstructionFirstUnapplied()
	f.conn.SendProto(6, burstDone)

	// Send an instruction chosen message for all chosen instructions above
	// our first unapplied point.
	instructions := store.InstructionSlots()
	firstUnapplied := store.InstructionFirstUnapplied()
	relativeSlot := int(firstUnapplied - store.InstructionStart())
	for ; relativeSlot < len(instructions); relativeSlot++ {
		slot := store.InstructionStart() + uint64(relativeSlot)
		slotValues := instructions[relativeSlot]
		if len(slotValues) != 1 || !slotValues[0].IsChosen() {
			continue
		}

		// Convert the change request to our internal format.
		chosen := new(fproto.InstructionChosen)
		chosen.Slot = new(uint64)
		*chosen.Slot = slot
		f.conn.SendProto(7, chosen)
	}
}

func sendInstructionData(f *followConn, slot uint64, req *store.ChangeRequest) {

	intReq := new(fproto.ChangeRequest)
	intReq.RequestEntity = new(uint64)
	intReq.RequestNode = new(uint32)
	intReq.RequestId = new(uint64)
	*intReq.RequestEntity = req.RequestEntity
	*intReq.RequestNode = uint32(req.RequestNode)
	*intReq.RequestId = req.RequestId
	intReq.Changeset = make([]*fproto.Change, len(req.Changeset))

	for i := range req.Changeset {
		intCh := new(fproto.Change)
		intCh.TargetEntity = new(uint64)
		intCh.Key = new(string)
		intCh.Value = new(string)
		*intCh.TargetEntity = req.Changeset[i].TargetEntity
		*intCh.Key = req.Changeset[i].Key
		*intCh.Value = req.Changeset[i].Value
		intReq.Changeset[i] = intCh
	}

	// Create and send an instruction data message.
	var dataMsg fproto.InstructionData
	dataMsg.Slot = new(uint64)
	*dataMsg.Slot = slot
	dataMsg.Request = intReq
	f.conn.SendProto(9, &dataMsg)
}
