package listener

import (
	"math/rand"
	"strconv"
	"time"
)

import (
	"github.com/jbeshir/unanimity/config"
	"github.com/jbeshir/unanimity/shared/chrequest"
	"github.com/jbeshir/unanimity/shared/store"
)

// Must only be accessed during a transaction.
var orphanAttachTimeouts map[uint64]*time.Timer
var namelessRemoveTimeouts map[uint64]*time.Timer

func init() {
	orphanAttachTimeouts = make(map[uint64]*time.Timer)
	namelessRemoveTimeouts = make(map[uint64]*time.Timer)
}

// Checks for "attach <id>" entries attaching this node ID to sessions we lack
// a connection to. Starts a timer to delete them if present.
// Must be called during a transaction, holding the session lock.
func checkOrphanAttaches() {
	attachedTo := store.AllAttachedTo(uint64(config.Id()))

	for _, session := range attachedTo {
		if sessions[session] == nil {
			startOrphanTimeout(session)
		}
	}
}

// Checks for nameless users in the store.
// If present, starts a timer to delete them.
// Must be called during a transaction.
func checkNameless() {
	ids := store.Nameless("user", "name username")
	for _, id := range ids {
		startNamelessTimeout(id)
	}
}

// Must be called during a transaction, holding the session lock.
func startOrphanTimeout(id uint64) {
	if orphanAttachTimeouts[id] == nil {
		delay := 2 * config.CHANGE_TIMEOUT_PERIOD
		timer := time.AfterFunc(delay, func() {
			orphanTimeout(id)
		})

		orphanAttachTimeouts[id] = timer
	}

}

// Must be called during a transaction.
func startNamelessTimeout(id uint64) {
	if namelessRemoveTimeouts[id] == nil {
		multiplier := time.Duration(rand.Intn(119) + 1)
		delay := multiplier * config.CHANGE_TIMEOUT_PERIOD
		timer := time.AfterFunc(delay, func() {
			namelessTimeout(id)
		})

		namelessRemoveTimeouts[id] = timer
	}
}

func orphanTimeout(id uint64) {
	store.StartTransaction()
	defer store.EndTransaction()

	// If the timeout has been removed, do nothing.
	if namelessRemoveTimeouts[id] == nil {
		return
	}

	// Detach ourselves from the session.
	// TODO: Should make sure we only do this once.
	ourAttachStr := "attach " + strconv.FormatUint(uint64(config.Id()), 10)
	session := store.GetEntity(id)
	if session != nil {
		chset := make([]store.Change, 1)
		chset[0].TargetEntity = id
		chset[0].Key = ourAttachStr
		chset[0].Value = ""

		req := makeRequest(chset)
		go chrequest.Request(req)
	}
}

func namelessTimeout(id uint64) {
	store.StartTransaction()
	defer store.EndTransaction()

	// If the timeout has been removed, do nothing.
	if namelessRemoveTimeouts[id] == nil {
		return
	}

	// Remove nameless user.
	// TODO: Should make sure we only do this once.
	user := store.GetEntity(id)
	if user != nil {
		chset := make([]store.Change, 1)
		chset[0].TargetEntity = id
		chset[0].Key = "id"
		chset[0].Value = ""

		req := makeRequest(chset)
		go chrequest.Request(req)
	}
}
