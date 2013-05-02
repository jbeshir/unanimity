/*
	Package implementing reliable storage of the global and entity stores,
	as well as reliable storage of both core and client node state
	requiring reliable persistence.

	Handles executing instructions once they are known to be chosen,
	as well as automatically removing earlier instructions.

	Provides a startup function, functions to read and set state,
	and transactions and locks for atomically performing sequences of
	operations. Provides callback registration functions for reacting
	to newly chosen instructions, applied instructions, changes of
	proposal number and leader node ID, and changes to degraded status.

	The callback for applied instructions provides the idempotent version
	of the instruction’s changeset.

	Client nodes do not use all the available state.
*/
package store

import (
	"log"
)

type Change struct {
	TargetEntity uint64
	Key          string
	Value        string
}

type ChangeRequest struct {
	RequestEntity uint64
	RequestNode   uint16
	RequestId     uint64
	Changeset     []Change
}

var transactionToken chan bool

var usedRequestIds uint64

func Startup() {
	transactionToken = make(chan bool, 1)
	transactionToken <- true

	// STUB
}

// Starts a transaction.
// Blocks until no other transaction is ongoing, then continues.
// Everything between a call to StartTransaction() and EndTransaction()
// will either be entirely applied to state, or not at all.
// All changes must occur inside a transaction.
// Changes must not be performed concurrently by multiple goroutines.
func StartTransaction() {
	<-transactionToken

	// STUB
	log.Print("starting transaction")
}

// Ends transaction. Changes made to the state are committed.
func EndTransaction() {
	// STUB

	transactionToken <- true
	log.Print("ending transaction")
}

// Returns a new request ID, never before used.
// Must be called inside a transaction.
func AllocateRequestId() uint64 {
	usedRequestIds++
	return usedRequestIds
}
