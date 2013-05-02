package store

import (
	"strconv"
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

	// Apply idempotent changesets.
	for i := range chrequests {
		applyChanges(chrequests[i].Changeset, true)
	}

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

// Applies the given set of changes.
// Returns an idempotent version of the changeset,
// whether or not it was given one or not.
func applyChanges(changes []Change, idempotent bool) []Change {

	// Set our new idempotent version of the changeset to the same slice
	// if we are to assume we were given an idempotent set,
	// and to a copy of it otherwise.
	var idemChanges []Change
	if idempotent {
		idemChanges = changes
	} else {
		idemChanges = append([]Change(nil), changes...)
	}

	// This type of loop lets us inject changes into the changeset.
	for i := 0; i < len(idemChanges); i++ {

		target := idemChanges[i].TargetEntity
		key := idemChanges[i].Key
		value := idemChanges[i].Value

		// If we're setting "id", it's creating an entity.
		if key == "id" && value != "" {

			// If this was not an idempotent changeset,
			// do needed special operation work.
			if !idempotent {

				// Get a new entity ID, and inject a change
				// incrementing the next entity ID.
				newIdStr := Global.Value("next entity")
				if newIdStr == "" {
					newIdStr = "65536"
				}

				newId, _ := strconv.ParseUint(newIdStr, 10, 64)
				nextNewStr := strconv.FormatUint(newId+1, 10)

				idemChanges = append(idemChanges, Change{})
				copy(idemChanges[i+2:], idemChanges[i+1:])
				idemChanges[i+1].Key = "next entity"
				idemChanges[i+1].Value = nextNewStr

				// Set this change to be creating that entity.
				idemChanges[i].TargetEntity = newId
				idemChanges[i].Value = newIdStr

				// Rewrite references to this ID everywhere
				// onwards in the changeset to refer to the
				// new ID. Include attach keys.
				old := target
				oldIdStr := strconv.FormatUint(target, 10)
				oldAttachKey := "attach " + oldIdStr
				newAttachKey := "attach " + newIdStr
				for j := i; j < len(idemChanges); j++ {
					if idemChanges[j].TargetEntity == old {
						idemChanges[j].TargetEntity =
							newId
					}

					if idemChanges[j].Key == oldAttachKey {
						idemChanges[j].Key =
							newAttachKey
					}
				}

				// Update our local variables.
				target = idemChanges[i].TargetEntity
				value = idemChanges[i].Value
			}

			// Create the entity, deleting any previous if needed.
			store := newStore()
			entityMap[target] = &store
			entityMap[target].values["id"] = value

			continue
		}

		// If we're clearing "id", it's deleting an entity.
		if key == "id" && value == "" {

			// If this was not an idempotent changeset,
			// do needed special operation work.
			if !idempotent {
				// TODO: Implement special operations.
			}

			// Delete the entity if it exists.
			// Call deleting callbacks first.
			if entityMap[target] != nil {
				for _, cb := range deletingCallbacks {
					cb(target)
				}

				delete(entityMap, target)
			}

			continue
		}

		// Otherwise, if the target doesn't exist, discard the change.
		// Should not happen in an already idempotent changeset.
		var store *Store
		if target != 0 {
			store = entityMap[target]
			if store == nil {
				idemChanges = append(idemChanges[:i],
					idemChanges[i+1:]...)

				// Don't increment our position.
				i--

				continue
			}
		} else {
			store = &Global
		}

		// If this changeset is already in an idempotent form,
		// everything else is just setting/unsetting keys.
		if !idempotent {
			// TODO: Implement special operations.
		}

		if value != "" {
			store.values[key] = value
		} else {
			delete(store.values, key)
		}
	}

	return idemChanges
}

// Returns whether the Paxos round represented by proposal1 and leader1 is
// higher than that represented by proposal2 and leader2.
// Does not read or write to store state.
func CompareProposals(proposal1 uint64, leader1 uint16,
	proposal2 uint64, leader2 uint16) bool {

	if proposal1 > proposal2 {
		return true
	} else if proposal1 == proposal2 && leader1 >= leader2 {
		return true
	}

	return false
}
