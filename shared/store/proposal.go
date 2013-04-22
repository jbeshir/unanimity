package store

import (
	"time"
)

import (
	"github.com/jbeshir/unanimity/config"
)

var proposal uint64
var leader uint16

var lastLeader time.Time

func Proposal() (proposal uint64, leader uint16) {
	return proposal, leader
}

func SetProposal(newProposal uint64, newLeader uint16) {
	if leader == config.Id() && newLeader != leader {
		lastLeader = time.Now()
	}

	proposal = newProposal
	leader = newLeader
}

func StoppedLeading() time.Time {
	return lastLeader
}
