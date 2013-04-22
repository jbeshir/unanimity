package store

import (
	"time"
)

import (
	"github.com/jbeshir/unanimity/config"
)

var degraded bool
var start uint64
var firstUnapplied uint64
var slots [][]*InstructionValue

type InstructionValue struct {
	slot           uint64
	latestProposal uint64
	latestLeader   uint16
	content        *ChangeRequest
	acceptedBy     []uint16
	chosen         bool
	chosenTime     time.Time
}

// Returns the proposal number and leader node ID that this
// instruction value was last accepted in.
func (i *InstructionValue) Proposal() (proposal uint64, leaderId uint16) {
	return i.latestProposal, i.latestLeader
}

// Returns the list of core node IDs which have accepted this instruction value.
// Must not be changed directly.
// Empty if IsChosen() is true.
func (i *InstructionValue) Accepted() []uint16 {
	return i.acceptedBy
}

// Returns whether this instruction value has been chosen.
func (i *InstructionValue) IsChosen() bool {
	return i.chosen
}

// Returns the time at which this instruction value was chosen.
func (i *InstructionValue) TimeChosen() time.Time {
	return i.chosenTime
}

// Returns the change request inside this instruction value.
// Is not copied, and so must not be changed.
func (i *InstructionValue) ChangeRequest() *ChangeRequest {
	return i.content
}

// Sets this instruction value as accepted by the given node ID.
// Safe to call more than once with the same node ID.
// If the given current proposal number and leader node ID are
// higher than our latest proposal number and leader node ID that
// this instruction was accepted in, we update our latest proposal number and
// leader node ID that this instruction was accepted in.
//
// Will automatically cause the instruction value to become chosen,
// with all other instruction values in the same slot cleared,
// once a majority of nodes have accepted it,
// and automatically cause it to be applied to state once all
// previous instruction slots have a chosen value.
func (i *InstructionValue) Accept(id uint16, proposal uint64, leader uint16) {
	if CompareProposals(proposal, id, i.latestProposal, i.latestLeader) {
		i.latestProposal = proposal
		i.latestLeader = leader
	}

	// If this value is already chosen, acceptance changes nothing.
	if i.chosen {
		return
	}

	// If this node ID already accepted this instruction value,
	// don't add it twice.
	for _, node := range i.acceptedBy {
		if node == id {
			return
		}
	}

	// Add this node ID to the accepting list.
	i.acceptedBy = append(i.acceptedBy, id)

	// If a majority of nodes have accepted this instruction value,
	// make it chosen.
	if len(i.acceptedBy) > len(config.CoreNodes())/2 {
		i.Choose()
	}
}

// Sets this instruction value as chosen, clearing all others in the same slot.
// Causes it to be automatically applied to state once all previous instruction
// slots have a chosen value.
func (i *InstructionValue) Choose() {

	relativeSlot := i.slot - start

	// Drop all other instruction values in the same slot.
	if len(slots[relativeSlot]) > 1 {
		slots[relativeSlot] = []*InstructionValue{i}
	}

	// Set the value as chosen.
	i.chosen = true
	i.chosenTime = time.Now()
	i.acceptedBy = nil

	// TODO: Handle the addition of a new chosen instruction.

	// Call instruction chosen callbacks.
	for _, cb := range chosenCallbacks {
		cb(i.slot)
	}
}

// Returns whether we are currently degraded.
func Degraded() bool {
	return degraded
}

// Returns the instruction slots.
// This slice is not copied and so must not be changed.
func InstructionSlots() [][]*InstructionValue {
	return slots
}

// Returns the number of the first instruction slot we have.
func InstructionStart() uint64 {
	return start
}

// Returns the number of the first unapplied instruction slot.
func InstructionFirstUnapplied() uint64 {
	return firstUnapplied
}

// Creates a new instruction value with the given change request,
// in the given slot. Returns nil if we can't add this instruction,
// because it is before the start of our slots,
// or we already have a chosen instruction for this slot.
func AddInstructionValue(slot uint64, req *ChangeRequest) *InstructionValue {

	// So named because it's a uint64, and we will be wanting an int later.
	relativeSlotBig := slot - start

	// Check that it isn't before the start of our instruction slots.
	if relativeSlotBig < 0 {
		return nil
	}

	// Check that it isn't too large to realistically store.
	// We set an arbitrary cap at 2^16 (65536) instructions.
	// This prevents the most egregious memory problems.
	if relativeSlotBig > 0xFFFF {
		return nil
	}
	relativeSlot := int(relativeSlotBig)

	// Make sure our instruction slots slice is big enough to store it.
	// If not, make it bigger.
	if relativeSlot > len(slots)-1 {
		// Increase instruction slots size by at least 50%.
		newSize := (len(slots) * 2) / 3
		if relativeSlot > newSize-1 {
			newSize = relativeSlot + 1
		}

		newSlots := make([][]*InstructionValue, newSize)
		copy(newSlots, slots)
		slots = newSlots
	}

	// If there is a chosen value in this slot,
	// we can't add more values.
	if len(slots[relativeSlot]) == 1 {
		existing := slots[relativeSlot][0]
		if existing.chosen {
			return nil
		}
	}

	// Otherwise, add a new value.
	i := new(InstructionValue)
	i.slot = slot
	i.content = req
	slots[relativeSlot] = append(slots[relativeSlot], i)
	return i
}

// Sets the node's degraded status to true, with all the consequences of that.
func Degrade() {
	degraded = true

	// Clear all data.
	Global = newStore()
	entityMap = make(map[uint64]*Store)
	for i := range slots {
		slots[i] = nil
	}

	// Run callbacks for when we change degraded status.
	for _, cb := range degradedCallbacks {
		cb()
	}
}

// Completes a burst. The change requests given are assumed to be idempotent,
// and are applied to state. The start instruction is set to the passed number.
// Takes the node out of degraded state.
func EndBurst(start uint64, chrequests []ChangeRequest) {
	if !degraded {
		panic("tried to set start instruction number while !degraded")
	}

	// TODO: Apply idempotent changesets.

	setInstructionStart(start)
	firstUnapplied = start
	degraded = false

	// Run callbacks for when we change degraded status.
	for _, cb := range degradedCallbacks {
		cb()
	}
}

// Sets the node's start instruction number.
// Called by both SetInstructionNumber,
// and the automatic removal of applied instructions.
func setInstructionStart(newStart uint64) {

	// If our first unapplied instruction would become in the past,
	// panic; this should never happen.
	if firstUnapplied < newStart {
		panic("tried to set start instruction number past " +
			"first unapplied instruction number")
	}

	// Copy down the contents of the slots buffer,
	// then clear the end of it.
	offset := int(newStart - start)
	if offset < len(slots) {
		copy(slots, slots[offset:])
		for i := len(slots) - offset; i < len(slots); i++ {
			slots[i] = nil
		}
	}

	// Set the new start instruction.
	start = newStart
}

// Returns whether the Paxos round represented by proposal1 and leader1 is
// higher than that represented by proposal2 and leader2.
// Does not read or write to store state.
func CompareProposals(proposal1 uint64, leader1 uint16,
	proposal2 uint64, leader2 uint16) bool {

	if proposal1 > proposal2 {
		return true
	} else if proposal1 == proposal2 && leader1 > leader2 {
		return true
	}

	return false
}
