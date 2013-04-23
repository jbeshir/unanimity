package consensus

import (
	"time"
)

import (
	"github.com/jbeshir/unanimity/config"
	"github.com/jbeshir/unanimity/shared/store"
)

// May only be accessed inside a transaction.
var proposalTimeouts map[uint64]*proposalTimeout

var proposalTimeoutCh chan *proposalTimeout

// May only be accessed inside a transaction.
var leaderTimeout *time.Timer

var leaderTimeoutCh chan uint64

func init() {
	proposalTimeouts = make(map[uint64]*proposalTimeout)
	proposalTimeoutCh = make(chan *proposalTimeout, 100)
	leaderTimeoutCh = make(chan uint64)
}

type proposalTimeout struct {
	slot  uint64
	req   *store.ChangeRequest
	timer *time.Timer
}

// Must be called inside a transaction.
func addProposalTimeout(slot uint64, req *store.ChangeRequest) {

	timeout := new(proposalTimeout)
	timeout.slot = slot
	timeout.req = req
	timeout.timer = time.AfterFunc(config.CHANGE_TIMEOUT_PERIOD,
		func() { proposalTimeoutCh <- timeout })

	proposalTimeouts[slot] = timeout
}

// Must be called inside a transaction.
// Cancels any proposal timeout which is in the same slot and is for a
// request with the same request node and request ID.
func cancelProposalTimeout(slot uint64, req *store.ChangeRequest) {

	timeout := proposalTimeouts[slot]
	if timeout == nil {
		return
	}

	if timeout.req.RequestNode != req.RequestNode {
		return
	}
	if timeout.req.RequestId != req.RequestId {
		return
	}

	timeout.timer.Stop()
	delete(proposalTimeouts, slot)
}

// Must be called inside a transaction.
func startLeaderTimeout(proposal uint64) {
	leaderTimeout = time.AfterFunc(config.ROUND_TRIP_TIMEOUT_PERIOD,
		func() { leaderTimeoutCh <- proposal })
}

// Must be called inside a transaction.
func stopLeaderTimeout() {
	if leaderTimeout != nil {
		leaderTimeout.Stop()
		leaderTimeout = nil
	}
}
