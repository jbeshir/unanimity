package store

var chosenCallbacks []func(slot uint64)
var deletingCallbacks []func(entityId uint64)
var appliedCallbacks []func(slot uint64, idempotentChanges []Change)
var proposalCallbacks []func()
var degradedCallbacks []func()

// Adds a callback to be called when a new instruction value is chosen.
// Called with the slot number of the chosen instruction value,
// after it has been added.
//
// The callback must not attempt to start a transaction,
// or mutate values held by the store package,
// and may assume no other changes have occurred since the change
// the callback corresponds to.
//
// May only be called prior to calling Startup(), usually from init().
func AddChosenCallback(cb func(slot uint64)) {
	chosenCallbacks = append(chosenCallbacks, cb)
}

// Adds a callback to be called when an entity is about to be deleting.
// Called with the entity ID of the entity being deleting, before deletion.
//
// The callback must not attempt to start a transaction,
// or mutate values held by the store package,
// and may assume no other changes have occurred since the change
// the callback corresponds to.
//
// May only be called prior to calling Startup(), usually from init().
func AddDeletingCallback(cb func(entityId uint64)) {
	deletingCallbacks = append(deletingCallbacks, cb)
}

// Adds a callback to be called when an instruction value is applied.
// Called with the slot number of the applied instruction value
// and an idempotent version of that instruction value’s changeset,
// as applied, after the instruction value is applied.
//
// The callback must not attempt to start a transaction,
// or mutate values held by the store package,
// and may assume no other changes have occurred since the change
// the callback corresponds to.
//
// May only be called prior to calling Startup(), usually from init().
func AddAppliedCallback(cb func(slot uint64, idempotentChanges []Change)) {
	appliedCallbacks = append(appliedCallbacks, cb)
}

// Adds a callback to be called when the current proposal number or
// leader node ID changes.
//
// The callback must not attempt to start a transaction,
// or mutate values held by the store package,
// and may assume no other changes have occurred since the change
// the callback corresponds to.
//
// May only be called prior to calling Startup(), usually from init().
func AddProposalCallback(cb func()) {
	proposalCallbacks = append(proposalCallbacks, cb)
}

// Adds a callback to be called when the degraded status of the state changes.
//
// The callback must not attempt to start a transaction,
// or mutate values held by the store package,
// and may assume no other changes have occurred since the change
// the callback corresponds to.
//
// May only be called prior to calling Startup(), usually from init().
func AddDegradedCallback(cb func()) {
	degradedCallbacks = append(degradedCallbacks, cb)
}
