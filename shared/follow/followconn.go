package follow

import (
	"sync"
	"time"
)

import (
	"github.com/jbeshir/unanimity/config"
	"github.com/jbeshir/unanimity/shared/connect"
	"github.com/jbeshir/unanimity/shared/follow/fproto"
	"github.com/jbeshir/unanimity/shared/store"
)

const firstUnappliedTimerDuration = config.CHANGE_TIMEOUT_PERIOD * 2

type followConn struct {
	// Constant once initialised.
	node     uint16
	conn     *connect.BaseConn
	outgoing bool

	// Changed from the processing goroutine.
	sendingBurst bool

	// Lock must be held to access anything below here.
	// Lock must be acquired after starting transaction, if both are done.
	lock                sync.Mutex
	offerTimers         map[uint64]*time.Timer
	receivingBurst      bool
	waiting             []*fproto.InstructionData
	closed              bool
	firstUnappliedTimer *time.Timer
}

// Closes the follow connection. May only be called with the connection's lock.
func (f *followConn) Close() {
	f.closed = true
	f.conn.Close()
}

func (f *followConn) firstUnappliedTimeout() {

	store.StartTransaction()
	defer store.EndTransaction()
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.closed {
		return
	}

	msg := new(fproto.FirstUnapplied)
	msg.FirstUnapplied = new(uint64)
	*msg.FirstUnapplied = store.InstructionFirstUnapplied()
	f.conn.SendProto(10, msg)

	f.firstUnappliedTimer = time.AfterFunc(firstUnappliedTimerDuration,
		func() { f.firstUnappliedTimeout() })

}

func (f *followConn) offerTimeout(slot uint64, duration time.Duration) {

	f.lock.Lock()
	defer f.lock.Unlock()

	if f.closed {
		return
	}

	// TODO: Only send this on one Follow connection.
	var intReq fproto.InstructionRequest
	intReq.Slot = new(uint64)
	*intReq.Slot = slot
	f.conn.SendProto(8, &intReq)

	f.offerTimers[slot] = time.AfterFunc(duration*2,
		func() { f.offerTimeout(slot, duration*2) })
}
