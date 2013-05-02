package store

import (
	"log"
	"time"
)

import (
	"github.com/jbeshir/unanimity/config"
)

var intProposal uint64
var intLeader uint16

var lastLeader time.Time

func Proposal() (proposal uint64, leader uint16) {
	return intProposal, intLeader
}

func SetProposal(newProposal uint64, newLeader uint16) {
	if intLeader == config.Id() && newLeader != intLeader {
		lastLeader = time.Now()
	}

	intProposal = newProposal
	intLeader = newLeader

	log.Print("shared/store: new proposal and leader ",
		intProposal, " ", intLeader)
}

// If the leader ID matches our ID,
// updates our "last stopped leading" time to now.
// Call when, whether or not we've a new leader,
// we consider ourselves to have stopped being the leader.
func StopLeading() {
	if intLeader == config.Id() {
		lastLeader = time.Now()
	}
}

func StoppedLeading() time.Time {
	return lastLeader
}
