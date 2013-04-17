package store

var proposal uint64
var leader uint16

func Proposal() (proposal uint64, leader uint16) {
	return proposal, leader
}

func SetProposal(newProposal uint64, newLeader uint16) {
	proposal = newProposal
	leader = newLeader
}
