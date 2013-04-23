package consensus

import (
	"github.com/jbeshir/unanimity/config"
	"github.com/jbeshir/unanimity/shared/chrequest"
	"github.com/jbeshir/unanimity/shared/store"
)

var newChangeCh chan *store.ChangeRequest

func init() {
	newChangeCh = make(chan *store.ChangeRequest, 100)

	store.AddChosenCallback(handleChosen)
	store.AddProposalCallback(handleProposalChange)
	chrequest.AddChangeCallback(handleChangeRequest)
}

func handleChosen(slot uint64) {
	relativeSlot := int(slot - store.InstructionStart())
	slots := store.InstructionSlots()

	cancelProposalTimeout(slot, slots[relativeSlot][0].ChangeRequest())
}

func handleProposalChange() {
	_, leader := store.Proposal()
	if amLeader && leader != config.Id() {
		stopBeingLeader()
	}
}

func handleChangeRequest(chrequest *store.ChangeRequest) {
	newChangeCh <- chrequest
}
