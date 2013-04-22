package chrequest

import (
	"github.com/jbeshir/unanimity/shared/store"
)

var changeCallbacks []func(chrequest *store.ChangeRequest)

// Adds a new callback to be called when the node determines that it wants to
// perform a change, as leader.
// May only be called on core nodes, and prior to calling Startup(),
// usually in init().
//
// Called within a shared/store transaction. May act on the store.
// Must not result in any calls to Request().
func AddChangeCallback(cb func(chrequest *store.ChangeRequest)) {
	changeCallbacks = append(changeCallbacks, cb)
}
